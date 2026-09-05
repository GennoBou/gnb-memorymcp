# AWS Lambda + Turso デプロイ手順書

本手順書は、Goで実装された共有記憶レイヤー（MCPサーバー）を AWS Lambda 上にデプロイし、Turso (libsql) データベースと連携させる手順を説明します。

---

## 1. 前提条件

- AWS アカウントおよび適切な権限（Lambda, IAM 等の操作権限）
- Turso アカウントおよび作成済みのデータベース
- ローカル環境における Go (v1.21以上) および Node.js/npm 開発環境
- Windows PowerShell 環境
- **AWS CLI v2** のインストールおよび認証情報 (`aws configure`) の設定

---

## 2. データベースの準備 (Turso)

> [!NOTE]
> `turso` CLI のログイン認証コマンドは Windows (PowerShell/CMD) では現在サポートされていません。
> Windows 環境で作業される場合は、**Turso Web ダッシュボード (https://turso.tech)** にログインし、Web 画面上からデータベースの作成と情報の取得を行うことを推奨します。
> （WSL または Linux/Mac 環境がある場合は、CLI もご利用いただけます）

### Web ダッシュボードから行う場合
1. [Turso Web ダッシュボード](https://turso.tech) にサインインします。
2. **[Create Database]** をクリックし、データベース（例: `gnb-memorymcp`）を作成します。
3. データベース詳細画面から **URL**（例: `libsql://gnb-memorymcp-username.turso.io`）をコピーします。
4. **[Generate Token]** ボタンをクリックして接続用トークンを発行し、コピーします。

### CLI (WSL や Linux/Mac 等) から行う場合
1. ターミナルでログインし、新しいデータベースを作成します。
   ```bash
   turso db create gnb-memorymcp
   ```
2. 接続用 URL を取得します。
   ```bash
   turso db show gnb-memorymcp --url
   ```
3. 認証用トークンを発行します。
   ```bash
   turso db tokens create gnb-memorymcp
   ```
4. 取得した **URL** と **トークン** を控えておきます（後ほど Lambda の環境変数に設定します）。

---

## 3. バイナリのビルドとパッケージング

Windows PowerShell 環境で、以下のコマンドを実行します。

```powershell
npm run build-lambda
```

- このコマンドは、内部的に `./build-lambda.ps1` を実行します。
- AWS Lambda のカスタムランタイム（`provided.al2023`）で動作させるため、`GOOS=linux GOARCH=amd64` 向けに `bootstrap` という名前でビルドされ、自動的に `bin/lambda.zip` に圧縮されます。

ビルドが完了すると、プロジェクトルート配下の `bin/` ディレクトリに `lambda.zip` が生成されます。

---

## 4. CLI によるワンコマンド・デプロイ（推奨）

事前に環境変数 (`DB_URL`, `DB_TOKEN`, `API_KEY`) を設定しておくことで、ビルドから Lambda へのアップロード・更新を一括で自動化できます。

### 4.1. 設定ファイルの作成 (.env.json)

環境変数をターミナル上で毎回指定する代わりに、プロジェクトルートに `.env.json` ファイルを作成して安全に設定を保存・管理できます（※ `.env.json` は `.gitignore` に登録されているため Git リポジトリにはコミットされません）。

サンプルファイル [`.env.json.example`](file:///d:/Data/Projects/GitHub/gnb-memorymcp/.env.json.example) をコピーして `.env.json` を作成し、ご自身の環境情報に書き換えてください。

**設定例 (`.env.json`)**:
```json
{
  "FUNCTION_NAME": "gnb-memorymcp",
  "REGION": "ap-northeast-3",
  "DB_URL": "libsql://gnb-memorymcp-username.turso.io",
  "DB_TOKEN": "your-turso-token",
  "API_KEY": "your-secure-api-key-here",
  "ALLOWED_ORIGINS": "https://example.com,https://app.example.com",
  "LAMBDA_ROLE_ARN": ""
}
```

※ `.env` ファイル (キー=値 形式) または PowerShell の環境変数 (`$env:DB_URL` 等) からの読み込みにも自動フォールバック対応しています。

### 4.2. ビルド & デプロイ実行

```powershell
npm run build-and-deploy-lambda
```

または個別実行:

```powershell
# デプロイのみ実行（lambda.zip が存在しない場合は自動ビルド）
npm run deploy-lambda
```

- **既存の Lambda 関数が存在する場合**: Zip コードおよび設定を最新に更新します。
- **新規作成する場合**: IAM ロール ARN の指定が必要です。`.env.json` の `LAMBDA_ROLE_ARN` に ARN を設定するか、引数でパラメータを渡します。

---

## 5. AWS SAM によるデプロイ（オプション）

AWS SAM CLI を利用した宣言的なインフラデプロイにも対応しています。

### 5.1. ビルドの実行
```powershell
npm run build-lambda
```

### 5.2. SAM によるデプロイ
```bash
sam deploy --guided
```

対話式プロンプトに従い、`DbUrl`, `DbToken`, `ApiKey` などのパラメータを入力することで、CloudFormation 経由でデプロイされます。

---

## 6. AWS 管理コンソールによる手動デプロイ

CLI を使用せず、Web コンソールから設定・デプロイを行う場合の手順です。

### 6.1. Lambda 関数の新規作成
1. AWS 管理コンソールにサインインし、**Lambda** サービスを開きます。
2. **[関数の作成]** をクリックします。
3. 以下の通り設定を行います：
   - **オプション**: 「一から作成」を選択します。
   - **関数名**: `gnb-memorymcp` (任意の名前)
   - **ランタイム**: **Amazon Linux 2023 上のカスタムランタイム (provided.al2023)**
   - **アーキテクチャ**: **x86_64** (ビルドしたバイナリの `GOARCH=amd64` と一致させます)
   - **実行ロール**: 既定の実行ロールの作成（基本的な Lambda 権限を持つロール）
4. **[関数の作成]** をクリックします。

### 6.2. コードのアップロード
1. 作成した関数の「コード」タブを開きます。
2. **[アップロード元]** ドロップダウンから **[.zip ファイル]** を選択します。
3. **[アップロード]** をクリックし、ローカルでビルドした `bin/lambda.zip` を選択してアップロードします。
4. **[保存]** をクリックします。

### 6.3. 環境変数の設定
1. 「設定」タブを開き、左メニューから **[環境変数]** を選択します。
2. **[編集]** をクリックし、以下の3つの環境変数を追加します：

   | キー | 値の例 | 説明 |
   | :--- | :--- | :--- |
   | `DB_URL` | `libsql://gnb-memorymcp-username.turso.io` | Turso の接続 URL |
   | `DB_TOKEN` | `eyJhbGciOiJ...` | Turso の認証用トークン |
   | `API_KEY` | `your-secure-api-key-here` | MCPサーバーの Bearer 認証用 API キー |
   | `ALLOWED_ORIGINS` | `https://example.com,https://app.example.com` | （任意）CORS 許可オリジンのカンマ区切りリスト |

3. **[保存]** をクリックします。

### 6.4. 関数 URL (Function URL) の有効化
1. 「設定」タブを開き、左メニューから **[関数 URL]** を選択します。
2. **[関数 URL の作成]** をクリックします。
3. **認証タイプ**: **NONE** を選択します。
4. **[保存]** をクリックします。表示される URL を接続先エンドポイントとして使用します。

---

## 7. 接続検証 (テスト)

ローカル環境のターミナル等から `curl` コマンドで動作を確認します。

```bash
curl -X POST <FUNCTION_URL> \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'
```

---

## 8. 各種 MCP クライアントでの設定方法

詳細は [docs/lambda_deployment.md](file:///d:/Data/Projects/GitHub/gnb-memorymcp/docs/lambda_deployment.md) または README を参照してください。
