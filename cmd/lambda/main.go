package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gennobou/gnb-memorymcp/pkg/auth"
	"github.com/gennobou/gnb-memorymcp/pkg/cors"
	"github.com/gennobou/gnb-memorymcp/pkg/infra/sqlite"
	"github.com/gennobou/gnb-memorymcp/pkg/mcp"
)

var (
	mcpHandler    *mcp.Handler
	apiKey        string
	auth0Domain   string
	auth0ClientID string
	initOnce      sync.Once
)

const (
	defaultProtocolVersion = "2026-07-28"
	defaultSessionID       = "session_gnb_memorymcp_active"
	fallbackLambdaURL      = "https://oajuohz4tofchqb4hy62yzq65m0paryw.lambda-url.ap-northeast-3.on.aws"
)

func initialize() {
	// 標準出力（stdout）を汚さないため、すべてのログは標準エラー出力（stderr）に出力します
	log.SetOutput(os.Stderr)

	dbURL := os.Getenv("DB_URL")
	dbToken := os.Getenv("DB_TOKEN")
	apiKey = os.Getenv("API_KEY")
	auth0Domain = os.Getenv("AUTH0_DOMAIN")
	if auth0Domain == "" {
		auth0Domain = "gennobou.jp.auth0.com"
	}
	auth0ClientID = os.Getenv("AUTH0_CLIENT_ID")

	if dbURL == "" {
		log.Fatalf("環境変数 DB_URL が設定されていません。")
	}

	if apiKey == "" {
		log.Fatalf("環境変数 API_KEY が設定されていません。")
	}

	// Turso (libsql) データベースへの接続初期化
	store, err := sqlite.NewStore(dbURL, dbToken)
	if err != nil {
		log.Fatalf("ストアの初期化に失敗しました: %v", err)
	}

	mcpHandler = mcp.NewHandler(store)
	log.Println("AWS Lambda 用エントリーポイントの初期化が完了しました。")
}

// HandleRequest は AWS Lambda への Function URL / API Gateway HTTP API からのリクエストを処理します
func HandleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	initOnce.Do(initialize)

	path := req.RawPath
	if path == "" {
		path = req.RequestContext.HTTP.Path
	}
	method := req.RequestContext.HTTP.Method

	log.Printf("[DEBUG-RAW] Path: %s, Method: %s, Headers: %+v", path, method, req.Headers)

	bodyBytes := decodeRequestBody(req)
	log.Printf("[DEBUG-RAW] Body: %s", string(bodyBytes))

	// 1. HTTP プリフライト / HEAD / DELETE リクエスト処理
	switch method {
	case "HEAD":
		cors := getCORSHeaders(req.Headers)
		cors["Access-Control-Allow-Methods"] = "POST, GET, HEAD, OPTIONS"
		cors["Access-Control-Allow-Headers"] = "*"
		cors["Mcp-Protocol-Version"] = defaultProtocolVersion
		return buildResponse(http.StatusOK, "", cors), nil
	case "OPTIONS":
		cors := getCORSHeaders(req.Headers)
		cors["Access-Control-Allow-Methods"] = "POST, GET, HEAD, OPTIONS"
		cors["Access-Control-Allow-Headers"] = "*"
		return buildResponse(http.StatusOK, "", cors), nil
	case "DELETE":
		return jsonResponse(req.Headers, http.StatusOK, map[string]string{"status": "deleted"}, nil), nil
	}

	// 2. ディスカバリエンドポイント (.well-known/*) 処理
	if resp, handled, err := handleDiscoveryEndpoints(path, req.Headers); handled {
		return resp, err
	}

	// 3. OAuth 2.0 / CIMD 認可ハンドラー (/authorize, /token, /register)
	if resp, handled, err := handleAuthEndpoints(path, req.QueryStringParameters, bodyBytes, req.Headers); handled {
		return resp, err
	}

	// 4. GET リクエスト（SSE / ヘルスチェック）
	if method == "GET" {
		return handleGETRequest(path, req.Headers), nil
	}

	// 5. POST リクエスト (MCP プロトコル処理)
	return handleMCPRequest(ctx, path, req.Headers, bodyBytes), nil
}

func getHeaderValue(headers map[string]string, key string) string {
	if val, ok := headers[strings.ToLower(key)]; ok && val != "" {
		return val
	}
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func getCORSHeaders(reqHeaders map[string]string) map[string]string {
	origin := getHeaderValue(reqHeaders, "Origin")
	allowedOriginsEnv := os.Getenv("ALLOWED_ORIGINS")
	ao := cors.ParseAllowedOrigins(allowedOriginsEnv)
	if ao == nil {
		return map[string]string{}
	}
	return ao.GetCORSHeaders(origin)
}

func getBaseURL(headers map[string]string) string {
	host := getHeaderValue(headers, "host")
	if host != "" {
		return "https://" + host
	}
	return fallbackLambdaURL
}

func decodeRequestBody(req events.APIGatewayV2HTTPRequest) []byte {
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(req.Body)
		if err == nil {
			return decoded
		}
	}
	return []byte(req.Body)
}

func buildResponse(statusCode int, body string, extraHeaders map[string]string) events.APIGatewayV2HTTPResponse {
	headers := map[string]string{}
	for k, v := range extraHeaders {
		headers[k] = v
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}
}

func jsonResponse(reqHeaders map[string]string, statusCode int, body interface{}, extraHeaders map[string]string) events.APIGatewayV2HTTPResponse {
	headers := map[string]string{
		"Content-Type":                 "application/json",
		"Access-Control-Allow-Headers": "*",
	}
	for k, v := range getCORSHeaders(reqHeaders) {
		headers[k] = v
	}
	for k, v := range extraHeaders {
		headers[k] = v
	}

	var bodyStr string
	switch v := body.(type) {
	case string:
		bodyStr = v
	case []byte:
		bodyStr = string(v)
	default:
		b, _ := json.Marshal(v)
		bodyStr = string(b)
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       bodyStr,
	}
}

func errorResponse(reqHeaders map[string]string, statusCode int, message string) events.APIGatewayV2HTTPResponse {
	headers := map[string]string{
		"Content-Type":                 "text/plain",
		"Access-Control-Allow-Headers": "*",
	}
	for k, v := range getCORSHeaders(reqHeaders) {
		headers[k] = v
	}

	if statusCode == http.StatusUnauthorized {
		headers["WWW-Authenticate"] = fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-authorization-server", error="unauthorized"`, fallbackLambdaURL)
		headers["Access-Control-Expose-Headers"] = "WWW-Authenticate, Mcp-Protocol-Version"
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       message,
	}
}

func handleDiscoveryEndpoints(path string, headers map[string]string) (events.APIGatewayV2HTTPResponse, bool, error) {
	baseURL := getBaseURL(headers)

	switch path {
	case "/.well-known/mcp", "/.well-known/mcp-configuration":
		return jsonResponse(headers, http.StatusOK, map[string]string{
			"name":            "gnb-memorymcp",
			"version":         "1.0.0",
			"protocolVersion": defaultProtocolVersion,
		}, map[string]string{"Mcp-Protocol-Version": defaultProtocolVersion}), true, nil

	case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
		metaJSON, err := auth.GetCIMDOAuthMetadataJSON(baseURL)
		if err != nil {
			return errorResponse(headers, http.StatusInternalServerError, "failed to generate oauth metadata"), true, nil
		}
		return jsonResponse(headers, http.StatusOK, metaJSON, nil), true, nil

	case "/.well-known/oauth-protected-resource":
		meta := map[string]interface{}{
			"resource":              baseURL,
			"authorization_servers": []string{baseURL},
			"scopes_supported":      []string{"mcp", "openid"},
		}
		return jsonResponse(headers, http.StatusOK, meta, nil), true, nil

	case "/.well-known/jwks.json":
		return jsonResponse(headers, http.StatusOK, map[string]interface{}{"keys": []interface{}{}}, nil), true, nil
	}

	return events.APIGatewayV2HTTPResponse{}, false, nil
}

func handleAuthEndpoints(path string, queryParams map[string]string, bodyBytes []byte, headers map[string]string) (events.APIGatewayV2HTTPResponse, bool, error) {
	switch path {
	case "/authorize":
		redirectURI := queryParams["redirect_uri"]
		state := queryParams["state"]
		if redirectURI == "" {
			return errorResponse(headers, http.StatusBadRequest, "missing redirect_uri"), true, nil
		}

		// AUTH0_CLIENT_ID が設定されている場合は Auth0 Universal Login 画面へリダイレクト
		if auth0ClientID != "" {
			baseURL := getBaseURL(headers)
			callbackURL := baseURL + "/callback"
			relayState, err := auth.EncodeRelayState(redirectURI, state)
			if err != nil {
				return errorResponse(headers, http.StatusInternalServerError, "failed to encode relay state"), true, nil
			}
			auth0URL := fmt.Sprintf(
				"https://%s/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=openid%%20profile%%20email&state=%s",
				auth0Domain,
				url.QueryEscape(auth0ClientID),
				url.QueryEscape(callbackURL),
				url.QueryEscape(relayState),
			)
			cors := getCORSHeaders(headers)
			cors["Location"] = auth0URL
			return buildResponse(http.StatusFound, "", cors), true, nil
		}

		targetURL, err := auth.BuildAuthorizeRedirectURL(redirectURI, state)
		if err != nil {
			return errorResponse(headers, http.StatusInternalServerError, "failed to generate authorize redirect url"), true, nil
		}
		cors := getCORSHeaders(headers)
		cors["Location"] = targetURL
		return buildResponse(http.StatusFound, "", cors), true, nil

	case "/callback":
		code := queryParams["code"]
		state := queryParams["state"]
		if code == "" {
			return errorResponse(headers, http.StatusBadRequest, "missing auth0 code"), true, nil
		}

		origRedirectURI, origState, err := auth.DecodeRelayState(state)
		if err != nil || origRedirectURI == "" {
			return errorResponse(headers, http.StatusBadRequest, "invalid or expired state parameter"), true, nil
		}

		// ログイン成功を確認し、Antigravity のコールバック先へ認可コードを発行してリダイレクト
		targetURL, err := auth.BuildAuthorizeRedirectURL(origRedirectURI, origState)
		if err != nil {
			return errorResponse(headers, http.StatusInternalServerError, "failed to generate redirect url"), true, nil
		}
		cors := getCORSHeaders(headers)
		cors["Location"] = targetURL
		return buildResponse(http.StatusFound, "", cors), true, nil

	case "/token":
		tokenJSON, err := auth.IssueCIMDToken()
		if err != nil {
			return errorResponse(headers, http.StatusInternalServerError, "failed to issue token"), true, nil
		}
		return jsonResponse(headers, http.StatusOK, tokenJSON, nil), true, nil

	case "/register":
		dcrJSON, err := auth.IssueDCRRegistrationResponse(bodyBytes)
		if err != nil {
			return errorResponse(headers, http.StatusInternalServerError, "failed to issue dcr client"), true, nil
		}
		return jsonResponse(headers, http.StatusCreated, dcrJSON, nil), true, nil
	}

	return events.APIGatewayV2HTTPResponse{}, false, nil
}

func handleGETRequest(path string, headers map[string]string) events.APIGatewayV2HTTPResponse {
	acceptHeader := getHeaderValue(headers, "accept")

	// MCP Streamable HTTP 仕様:
	// ステートレスサーバー（Lambda 等）で Standalone SSE Stream を提供しない場合は
	// GET (text/event-stream) や /sse に対して 405 Method Not Allowed (Allow: POST, GET, HEAD, OPTIONS) を返す
	if path == "/sse" || strings.Contains(acceptHeader, "text/event-stream") {
		cors := getCORSHeaders(headers)
		cors["Allow"] = "POST, GET, HEAD, OPTIONS"
		cors["Content-Type"] = "text/plain"
		cors["Mcp-Protocol-Version"] = defaultProtocolVersion
		cors["Access-Control-Expose-Headers"] = "Mcp-Protocol-Version, Allow"
		return buildResponse(http.StatusMethodNotAllowed, "Standalone SSE stream not supported in stateless mode", cors)
	}

	return jsonResponse(headers, http.StatusOK, map[string]interface{}{
		"name":            "GNB MemoryMCP",
		"version":         "1.0.0",
		"protocolVersion": defaultProtocolVersion,
		"status":          "ok",
		"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
	}, map[string]string{
		"Mcp-Protocol-Version": defaultProtocolVersion,
		"Mcp-Session-Id":       defaultSessionID,
	})
}

func handleMCPRequest(ctx context.Context, path string, headers map[string]string, bodyBytes []byte) events.APIGatewayV2HTTPResponse {
	var mcpReq mcp.Request
	if err := json.Unmarshal(bodyBytes, &mcpReq); err != nil {
		log.Printf("[DEBUG] json.Unmarshal error: %v\n", err)
		resp := mcp.NewErrorResponse(nil, mcp.CodeParseError, "parse error")
		return jsonResponse(headers, http.StatusOK, resp, nil)
	}

	// initialize メソッドは未認証でも応答可能
	if mcpReq.Method == "initialize" {
		mcpResp := mcpHandler.Handle(ctx, &mcpReq)
		protoVersion := defaultProtocolVersion
		if resMap, ok := mcpResp.Result.(map[string]interface{}); ok {
			if pv, ok := resMap["protocolVersion"].(string); ok && pv != "" {
				protoVersion = pv
			}
		}
		return jsonResponse(headers, http.StatusOK, mcpResp, map[string]string{
			"Mcp-Protocol-Version":          protoVersion,
			"Mcp-Session-Id":                defaultSessionID,
			"Access-Control-Expose-Headers": "Mcp-Protocol-Version, Mcp-Session-Id, WWW-Authenticate",
		})
	}

	// notifications/initialized (204 No Content)
	if mcpReq.Method == "notifications/initialized" {
		cors := getCORSHeaders(headers)
		cors["Mcp-Session-Id"] = defaultSessionID
		return buildResponse(http.StatusNoContent, "", cors)
	}

	// 認証チェック
	authHeader := getHeaderValue(headers, "authorization")
	log.Printf("[DEBUG] HandleRequest Path: %s, HasAuthHeader: %v", path, authHeader != "")

	token, err := auth.ExtractBearerToken(authHeader)
	if err != nil {
		log.Printf("[DEBUG] ExtractBearerToken error: %v", err)
		return errorResponse(headers, http.StatusUnauthorized, "Unauthorized: "+err.Error())
	}

	verifier := auth.NewMultiVerifier(
		auth.NewApiKeyVerifier(apiKey),
		auth.NewAuth0UserInfoVerifier(auth0Domain),
		auth.NewJWTBearerVerifier(auth0Domain),
		auth.NewCIMDBearerVerifier(),
	)
	if err := verifier.VerifyToken(ctx, token); err != nil {
		log.Printf("[DEBUG] VerifyToken error: %v", err)
		return errorResponse(headers, http.StatusUnauthorized, "Unauthorized: invalid token")
	}

	mcpResp := mcpHandler.Handle(ctx, &mcpReq)
	if mcpResp == nil {
		cors := getCORSHeaders(headers)
		return buildResponse(http.StatusNoContent, "", cors)
	}

	return jsonResponse(headers, http.StatusOK, mcpResp, map[string]string{
		"Mcp-Protocol-Version": defaultProtocolVersion,
		"Mcp-Session-Id":       defaultSessionID,
	})
}

func main() {
	lambda.Start(HandleRequest)
}
