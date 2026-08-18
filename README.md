# GNB MemoryMCP

複数の LLM や AI エージェント（ChatGPT, Claude, Claude Code, Gemini, Antigravity など）から共通で参照・更新できる、ベンダー非依存の個人用記憶層（MCP サーバー）です。

本プロジェクトは Go で実装されており、ローカル環境（SQLite）および本番環境（AWS Lambda + Turso / libSQL）の双方でシームレスに切り替えて動作するように設計されています。

---

## 特徴

- **複数LLM間での記憶共有**: 特定のベンダーにロックインされることなく、あなたの設定やプロジェクトの文脈、知識をすべての AI に引き継ぐことができます。
- **2種類のローカル接続に対応**:
  1. **HTTP サーバーモード**: ポートを指定して起動し、複数のクライアントからアクセスします。認証には `Authorization: Bearer <API_KEY>` を使用します。
  2. **標準入出力 (stdio) モード**: ポートを開かず、Claude Desktop 等の MCP クライアントから直接バイナリを実行・通信させます。
- **全文検索 (FTS5)**: SQLite の FTS5 を用いた高速かつ軽量なテキスト全文検索に対応しています。
- **スキーマの永続性**: `tags` や `metadata` を JSON として格納するため、API 仕様を変更することなく、将来的にベクトル検索や要約バッチなどの内部実装を進化させることができます。

---

## ディレクトリ構成

- `cmd/gnb-memorymcp/`: ローカル統合バイナリ（`server` / `stdio` モード）のエントリーポイント
- `cmd/lambda/`: AWS Lambda 起動用のエントリーポイント
- `pkg/auth/`: Bearer 認証、JWT検証、および CIMD クライアントID 判定
- `pkg/domain/`: 記憶モデルおよびデータベースストアのインターフェース定義
- `pkg/infra/sqlite/`: SQLite / libSQL による DB 保存・全文検索 (FTS5)・トリガー同期の実装
- `pkg/mcp/`: JSON-RPC 2.0 に準拠した MCP ツール実行のハンドラー

---

## インストール方法

### Scoop によるインストール (Windows 推奨)

```powershell
# バケットの追加
scoop bucket add gnb-bucket https://github.com/GennoBou/scoop-bucket

# インストール
scoop install gnb-memorymcp
```

### ソースコードからのビルド

```bash
# 全部入りバイナリ（ローカル用）のビルド
go build -o bin/gnb-memorymcp.exe ./cmd/gnb-memorymcp/main.go
```

---

## 使用方法

### 1. 標準入出力 (stdio) モードの設定例
もっとも手軽にローカルで MCP を動作させる方法です。接続するクライアント（Claude Desktop、Antigravity、OpenCode など）の構成ファイルに直接バイナリを登録します。明示的に第1引数に `stdio` を渡すことで、標準入出力モードとして起動します。

#### A. Claude Desktop の場合
設定ファイル: `AppData\Roaming\Claude\claude_desktop_config.json` (Windows)

```json
{
  "mcpServers": {
    "gnb-memorymcp": {
      "command": "gnb-memorymcp.exe",
      "args": ["stdio", "--db-url", "file:D:/Data/Projects/GitHub/gnb-memorymcp/data/local_v2.db"]
    }
  }
}
```

#### B. Antigravity の場合
設定ファイル: `~/.gemini/antigravity-cli/settings.json` (Windows の場合は `C:\Users\<ユーザー名>\.gemini\antigravity-cli\settings.json`)

```json
{
  "mcpServers": {
    "gnb-memorymcp": {
      "command": "gnb-memorymcp.exe",
      "args": ["stdio", "--db-url", "file:D:/Data/Projects/GitHub/gnb-memorymcp/data/local_v2.db"]
    }
  }
}
```

#### C. OpenCode の場合
設定ファイル: `~/.config/opencode/opencode.jsonc`

```jsonc
{
  "mcp": {
    "gnb-memorymcp": {
      "type": "local",
      "enabled": true,
      "command": [
        "gnb-memorymcp.exe",
        "stdio",
        "--db-url",
        "file:D:/Data/Projects/GitHub/gnb-memorymcp/data/local_v2.db"
      ]
    }
  }
}
```

設定を反映後、対象のクライアントを再起動することで、記憶ツール（`memory_create` 等）が自動的に読み込まれ、使用可能になります。

---

### 2. HTTP サーバーモード（ローカル SQLite）
ローカルまたは別端末で HTTP サーバーとして起動し、ローカルの SQLite データベースを参照します。第1引数に `server` を渡して起動します。
安全のため、デフォルトではローカルホスト（`127.0.0.1`）にバインドされます。

**起動方法 (PowerShell)**

- **方法1: コマンド引数で指定して起動**
  ```powershell
  # ローカルホストからのみ接続可能として起動 (APIキーが未指定の場合は自動的に 'dev-key' が使われます)
  ./bin/gnb-memorymcp.exe server --port 8080 --db-url "file:data/local_v2.db"

  # 外部ネットワークに公開して起動 (--host 0.0.0.0 指定時は、安全な独自の --api-key 設定が必須)
  ./bin/gnb-memorymcp.exe server --host 0.0.0.0 --port 8080 --api-key "your-secure-secret-key" --db-url "file:data/local_v2.db"
  ```

- **方法2: 環境変数で指定して起動**
  ```powershell
  $env:HOST="127.0.0.1"
  $env:API_KEY="your-secret-key"
  $env:PORT="8080"
  $env:DB_URL="file:data/local_v2.db"
  ./bin/gnb-memorymcp.exe server
  ```

**クライアントからのリクエスト方法**
- URL: `http://localhost:8080/`
- メソッド: `POST`
- 必須ヘッダー: `Authorization: Bearer your-secret-key`
- リクエストボディ (JSON-RPC 2.0):
  ```json
  {
    "jsonrpc": "2.0",
    "method": "tools/list",
    "id": 1
  }
  ```

---

### 3. ローカル HTTP + Turso モード（クラウド DB 連携）
ローカルで HTTP サーバーを起動し、データベースのみリモートの Turso (libSQL) を参照させます。

**起動方法 (PowerShell)**

- **方法1: コマンド引数で指定して起動**
  ※ `DB_TOKEN` はセキュリティ保護のため環境変数への設定を推奨します。
  ```powershell
  $env:DB_TOKEN="your-turso-auth-token"
  ./bin/gnb-memorymcp.exe server --port 8080 --api-key "your-secret-key" --db-url "libsql://your-database-name-username.turso.io"
  ```

- **方法2: 環境変数で指定して起動**
  ```powershell
  $env:API_KEY="your-secret-key"
  $env:PORT="8080"
  $env:DB_URL="libsql://your-database-name-username.turso.io"
  $env:DB_TOKEN="your-turso-auth-token"
  ./bin/gnb-memorymcp.exe server
  ```

---

### 4. AWS Lambda + Turso モード（本番クラウド運用）
AWS Lambda 上にサーバーレスでデプロイし、Turso と連携して動作させる構成です。
- エントリーポイント: `cmd/lambda/main.go`
- ワンコマンドでのビルド＆デプロイスクリプト（`npm run build-and-deploy-lambda`）や SAM テンプレート（`template.yaml`）に対応しています。

詳細なデプロイ手順については、[AWS Lambda + Turso デプロイ手順書](file:///d:/Data/Projects/GitHub/gnb-memorymcp/docs/lambda_deployment.md) を参照してください。


---

## 提供する MCP ツール仕様

GNB MemoryMCP は以下の 10 個のツールを提供します。

| ツール名 | 説明 | 主要な引数 |
|---|---|---|
| `memory_create` | 新規記憶を保存します（最大10,000文字、タグ数最大10、重要度0〜10）。 | `content`, `source_tool`, `tags`, `importance`, `metadata` |
| `memory_search` | 日本語 FTS5 trigram および LIKE によるハイブリッド想起。重要度と鮮度で再ランキング。 | `query`, `top_k` (最大50) |
| `memory_list` | 記憶の一覧を取得します。ソート・ページネーション指定に対応。 | `source_tool`, `tag`, `limit` (最大100), `offset`, `sort_by`, `order` |
| `memory_get` | ID指定で記憶を1件取得します。 | `id` |
| `memory_update` | 既存の記憶を差分更新します（空値による削除クリア・楽観的ロックに対応）。 | `id`, `content`, `source_tool`, `tags`, `importance`, `metadata` |
| `memory_delete` | 不要になった記憶を ID 指定で物理削除します。 | `id` |
| `memory_status` | データベースの総件数や、クレンジング（整理）の必要性を確認します。 | なし |
| `memory_consolidate` | 重複・矛盾している可能性のある記憶のペアを軽量抽出（類似度走査の高速化）します。 | `limit` (最大50), `offset` |
| `memory_cleanup_complete`| クレンジング完了を記録し、最終整理日時を更新します。 | なし |
| `tags_list` | これまでに登録されたユニークなタグの一覧を取得します。 | なし |
