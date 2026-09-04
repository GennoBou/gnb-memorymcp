package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gennobou/gnb-memorymcp/pkg/auth"
	"github.com/gennobou/gnb-memorymcp/pkg/infra/sqlite"
	"github.com/gennobou/gnb-memorymcp/pkg/mcp"
)

var (
	mcpHandler  *mcp.Handler
	apiKey      string
	auth0Domain string
	initOnce    sync.Once
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
		return buildResponse(http.StatusOK, "", map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST, GET, HEAD, OPTIONS",
			"Access-Control-Allow-Headers": "*",
			"Mcp-Protocol-Version":        defaultProtocolVersion,
		}), nil
	case "OPTIONS":
		return buildResponse(http.StatusOK, "", map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST, GET, HEAD, OPTIONS",
			"Access-Control-Allow-Headers": "*",
		}), nil
	case "DELETE":
		return jsonResponse(http.StatusOK, map[string]string{"status": "deleted"}, nil), nil
	}

	// 2. ディスカバリエンドポイント (.well-known/*) 処理
	if resp, handled, err := handleDiscoveryEndpoints(path, req.Headers); handled {
		return resp, err
	}

	// 3. OAuth 2.0 / CIMD 認可ハンドラー (/authorize, /token, /register)
	if resp, handled, err := handleAuthEndpoints(path, req.QueryStringParameters, bodyBytes); handled {
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

func jsonResponse(statusCode int, body interface{}, extraHeaders map[string]string) events.APIGatewayV2HTTPResponse {
	headers := map[string]string{
		"Content-Type":                 "application/json",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "*",
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

func errorResponse(statusCode int, message string) events.APIGatewayV2HTTPResponse {
	headers := map[string]string{
		"Content-Type":                 "text/plain",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "*",
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
		return jsonResponse(http.StatusOK, map[string]string{
			"name":            "gnb-memorymcp",
			"version":         "1.0.0",
			"protocolVersion": defaultProtocolVersion,
		}, map[string]string{"Mcp-Protocol-Version": defaultProtocolVersion}), true, nil

	case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
		metaJSON, err := auth.GetCIMDOAuthMetadataJSON(baseURL)
		if err != nil {
			return errorResponse(http.StatusInternalServerError, "failed to generate oauth metadata"), true, nil
		}
		return jsonResponse(http.StatusOK, metaJSON, nil), true, nil

	case "/.well-known/oauth-protected-resource":
		meta := map[string]interface{}{
			"resource":              baseURL,
			"authorization_servers": []string{baseURL},
			"scopes_supported":      []string{"mcp", "openid"},
		}
		return jsonResponse(http.StatusOK, meta, nil), true, nil

	case "/.well-known/jwks.json":
		return jsonResponse(http.StatusOK, map[string]interface{}{"keys": []interface{}{}}, nil), true, nil
	}

	return events.APIGatewayV2HTTPResponse{}, false, nil
}

func handleAuthEndpoints(path string, queryParams map[string]string, bodyBytes []byte) (events.APIGatewayV2HTTPResponse, bool, error) {
	switch path {
	case "/authorize":
		redirectURI := queryParams["redirect_uri"]
		state := queryParams["state"]
		if redirectURI == "" {
			return errorResponse(http.StatusBadRequest, "missing redirect_uri"), true, nil
		}
		targetURL, err := auth.BuildAuthorizeRedirectURL(redirectURI, state)
		if err != nil {
			return errorResponse(http.StatusInternalServerError, "failed to generate authorize redirect url"), true, nil
		}
		return buildResponse(http.StatusFound, "", map[string]string{
			"Location":                    targetURL,
			"Access-Control-Allow-Origin": "*",
		}), true, nil

	case "/token":
		tokenJSON, err := auth.IssueCIMDToken()
		if err != nil {
			return errorResponse(http.StatusInternalServerError, "failed to issue token"), true, nil
		}
		return jsonResponse(http.StatusOK, tokenJSON, nil), true, nil

	case "/register":
		dcrJSON, err := auth.IssueDCRRegistrationResponse(bodyBytes)
		if err != nil {
			return errorResponse(http.StatusInternalServerError, "failed to issue dcr client"), true, nil
		}
		return jsonResponse(http.StatusCreated, dcrJSON, nil), true, nil
	}

	return events.APIGatewayV2HTTPResponse{}, false, nil
}

func handleGETRequest(path string, headers map[string]string) events.APIGatewayV2HTTPResponse {
	acceptHeader := getHeaderValue(headers, "accept")
	baseURL := getBaseURL(headers)

	if path == "/sse" || (strings.Contains(acceptHeader, "text/event-stream") && !strings.Contains(acceptHeader, "application/json")) {
		return buildResponse(http.StatusOK, fmt.Sprintf("event: endpoint\ndata: %s/\n\n", baseURL), map[string]string{
			"Content-Type":                 "text/event-stream",
			"Cache-Control":                "no-cache",
			"Connection":                   "keep-alive",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Headers": "*",
		})
	}

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"name":            "GNB MemoryMCP",
		"version":         "1.0.0",
		"protocolVersion": "2025-11-25",
		"status":          "ok",
		"capabilities":   map[string]interface{}{"tools": map[string]interface{}{}},
	}, map[string]string{
		"Mcp-Protocol-Version": "2025-11-25",
		"Mcp-Session-Id":       defaultSessionID,
	})
}

func handleMCPRequest(ctx context.Context, path string, headers map[string]string, bodyBytes []byte) events.APIGatewayV2HTTPResponse {
	var mcpReq mcp.Request
	if err := json.Unmarshal(bodyBytes, &mcpReq); err != nil {
		log.Printf("[DEBUG] json.Unmarshal error: %v\n", err)
		resp := mcp.NewErrorResponse(nil, mcp.CodeParseError, "parse error")
		return jsonResponse(http.StatusOK, resp, nil)
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
		return jsonResponse(http.StatusOK, mcpResp, map[string]string{
			"Mcp-Protocol-Version":          protoVersion,
			"Mcp-Session-Id":                defaultSessionID,
			"Access-Control-Expose-Headers": "Mcp-Protocol-Version, Mcp-Session-Id, WWW-Authenticate",
		})
	}

	// notifications/initialized (204 No Content)
	if mcpReq.Method == "notifications/initialized" {
		return buildResponse(http.StatusNoContent, "", map[string]string{
			"Access-Control-Allow-Origin": "*",
			"Mcp-Session-Id":              defaultSessionID,
		})
	}

	// 認証チェック
	authHeader := getHeaderValue(headers, "authorization")
	log.Printf("[DEBUG] HandleRequest Path: %s, HasAuthHeader: %v", path, authHeader != "")

	token, err := auth.ExtractBearerToken(authHeader)
	if err != nil {
		log.Printf("[DEBUG] ExtractBearerToken error: %v", err)
		return errorResponse(http.StatusUnauthorized, "Unauthorized: "+err.Error())
	}

	verifier := auth.NewMultiVerifier(
		auth.NewApiKeyVerifier(apiKey),
		auth.NewAuth0UserInfoVerifier(auth0Domain),
		auth.NewJWTBearerVerifier(),
		auth.NewCIMDBearerVerifier(),
	)
	if err := verifier.VerifyToken(ctx, token); err != nil {
		log.Printf("[DEBUG] VerifyToken error: %v", err)
		return errorResponse(http.StatusUnauthorized, "Unauthorized: invalid token")
	}

	mcpResp := mcpHandler.Handle(ctx, &mcpReq)
	if mcpResp == nil {
		return buildResponse(http.StatusNoContent, "", nil)
	}

	return jsonResponse(http.StatusOK, mcpResp, map[string]string{
		"Mcp-Protocol-Version": defaultProtocolVersion,
		"Mcp-Session-Id":       defaultSessionID,
	})
}

func main() {
	lambda.Start(HandleRequest)
}
