package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gennobou/gnb-memorymcp/pkg/auth"
	"github.com/gennobou/gnb-memorymcp/pkg/infra/sqlite"
	"github.com/gennobou/gnb-memorymcp/pkg/mcp"
)

func main() {
	// 標準出力（stdout）を汚さないため、すべてのログは標準エラー出力（stderr）に出力します
	log.SetOutput(os.Stderr)

	// フラグの定義
	fs := flag.NewFlagSet("gnb-memorymcp", flag.ExitOnError)
	dbURLFlag := fs.String("db-url", "", "SQLite データベースファイルへのパス、または Turso URL")
	hostFlag := fs.String("host", "", "HTTP サーバーモード用のバインドホスト名 (127.0.0.1 または 0.0.0.0 など)")
	portFlag := fs.String("port", "", "HTTP サーバーモード用のポート番号")
	apiKeyFlag := fs.String("api-key", "", "HTTP サーバーモード認証用の API キー")
	allowedOriginsFlag := fs.String("allowed-origins", "", "CORS 許可オリジンのカンマ区切りリスト (例: https://example.com,http://localhost:3000)")

	// 第1引数がサブコマンド（stdio / server）であるかチェック
	mode := "stdio" // デフォルトは stdio（互換性確保のため）
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "stdio" || args[0] == "server") {
		mode = args[0]
		args = args[1:]
	}

	// 引数のパース
	if err := fs.Parse(args); err != nil {
		log.Fatalf("引数のパースに失敗しました: %v", err)
	}

	// データベース URL の決定 (引数 -> 環境変数 -> デフォルトの順)
	dbURL := *dbURLFlag
	if dbURL == "" {
		dbURL = os.Getenv("DB_URL")
		if dbURL == "" {
			dbURL = "file:data/local_v2.db"
		}
	}
	dbToken := os.Getenv("DB_TOKEN")

	store, err := sqlite.NewStore(dbURL, dbToken)
	if err != nil {
		log.Fatalf("ストアの初期化に失敗しました: %v", err)
	}
	defer store.Close()
	log.Printf("データベースを初期化しました: %s (起動モード: %s)", dbURL, mode)

	mcpHandler := mcp.NewHandler(store)
	ctx := context.Background()

	switch mode {
	case "stdio":
		runStdio(ctx, mcpHandler)
	case "server":
		// ホストの決定
		host := *hostFlag
		if host == "" {
			host = os.Getenv("HOST")
			if host == "" {
				host = "127.0.0.1" // デフォルトはローカルホストのみ
			}
		}

		// ポート番号の決定
		port := *portFlag
		if port == "" {
			port = os.Getenv("PORT")
			if port == "" {
				port = "8080"
			}
		}

		// API キーの決定
		apiKey := *apiKeyFlag
		if apiKey == "" {
			apiKey = os.Getenv("API_KEY")
		}

		// バリデーション: 0.0.0.0 (外部公開) の場合、APIキーの設定が必須
		if host == "0.0.0.0" {
			if apiKey == "" {
				log.Fatalf("エラー: ホストに 0.0.0.0 が指定されていますが、API_KEY が設定されていません。外部に公開されるため、APIキーの設定が必須です。")
			}
			if apiKey == "dev-key" {
				log.Fatalf("エラー: ホストに 0.0.0.0 が指定されていますが、API_KEY がデフォルト値 'dev-key' のままです。安全のため、0.0.0.0 バインド時は独自のセキュアなAPIキーを設定してください。")
			}
		}

		// デフォルト値のフォールバック (ローカルホストバインド時のみ)
		if apiKey == "" {
			apiKey = "dev-key"
			log.Printf("API_KEY が設定されていません。デフォルト値 'dev-key' を使用します (ローカルバインドのみ)")
		}

		allowedOrigins := *allowedOriginsFlag
		if allowedOrigins == "" {
			allowedOrigins = os.Getenv("ALLOWED_ORIGINS")
		}

		runHTTPServer(ctx, mcpHandler, host, port, apiKey, allowedOrigins)
	default:
		log.Fatalf("未知のモードです: %s", mode)
	}
}

// runStdio は標準入出力 (stdio) モードで MCP リクエストを処理します
func runStdio(ctx context.Context, mcpHandler *mcp.Handler) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req mcp.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			resp := mcp.NewErrorResponse(nil, mcp.CodeParseError, "parse error")
			sendResponse(resp)
			continue
		}

		resp := mcpHandler.Handle(ctx, &req)
		if resp != nil {
			sendResponse(resp)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("標準入力のスキャン中にエラーが発生しました: %v", err)
	}
}

// runHTTPServer は HTTP サーバーモードで MCP リクエストを処理します
func runHTTPServer(ctx context.Context, mcpHandler *mcp.Handler, host, port, apiKey, allowedOriginsStr string) {
	auth0Domain := os.Getenv("AUTH0_DOMAIN")
	if auth0Domain == "" {
		auth0Domain = "gennobou.jp.auth0.com"
	}

	allowedOrigins := parseAllowedOrigins(allowedOriginsStr)

	mux := http.NewServeMux()

	// OAuth 2.0 / OIDC Discovery ハンドラー
	discoveryHandler := makeDiscoveryHandler(auth0Domain, allowedOrigins)

	mux.HandleFunc("/.well-known/oauth-authorization-server", discoveryHandler)
	mux.HandleFunc("/.well-known/openid-configuration", discoveryHandler)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// リクエストボディのサイズ制限 (最大 10MB)
		r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

		// 認証ヘッダーの確認
		token, err := auth.ExtractBearerToken(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}
		verifier := auth.NewMultiVerifier(
			auth.NewApiKeyVerifier(apiKey),
			auth.NewAuth0UserInfoVerifier(auth0Domain),
		)
		if err := verifier.VerifyToken(r.Context(), token); err != nil {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		var req mcp.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mcp.NewErrorResponse(nil, mcp.CodeParseError, "parse error"))
			return
		}

		resp := mcpHandler.Handle(r.Context(), &req)
		if resp == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("レスポンスの書き込みに失敗しました: %v", err)
		}
	})

	addr := host + ":" + port
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("サーバーを %s で起動します...", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("サーバーの起動に失敗しました: %v", err)
	}
}

func parseAllowedOrigins(originsStr string) map[string]bool {
	if originsStr == "" {
		return nil
	}
	origins := make(map[string]bool)
	for _, o := range strings.Split(originsStr, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			origins[trimmed] = true
		}
	}
	return origins
}

func setCORSHeaders(w http.ResponseWriter, r *http.Request, allowedOrigins map[string]bool) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if allowedOrigins != nil && allowedOrigins[origin] {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
}

func makeDiscoveryHandler(auth0Domain string, allowedOrigins map[string]bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metaJSON, err := auth.GetOAuthMetadataJSON(auth0Domain)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		setCORSHeaders(w, r, allowedOrigins)
		_, _ = w.Write(metaJSON)
	}
}

// sendResponse はレスポンスを JSON 形式で標準出力に出力します
func sendResponse(resp *mcp.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("レスポンスのパースに失敗しました: %v", err)
		return
	}
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}
