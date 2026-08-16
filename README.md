# AP MCP Slack

[![CI](https://github.com/shouni/ap-mcp-slack/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/ap-mcp-slack/actions/workflows/ci.yml)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/ap-mcp-slack)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/ap-mcp-slack)](https://github.com/shouni/ap-mcp-slack/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Slack へのメッセージ投稿・更新・削除に加え、チャンネル・履歴・スレッドの読み取り、ワークスペース横断検索、ユーザー解決までをツールとして提供するMCPサーバーです。Slack Incoming Webhook と Slack Web API の2系統のトランスポートに対応します。

MCPクライアントからコマンドとして起動され、stdin/stdout の stdio transport で通信します。ローカルホストのHTTPサーバーやCloud Runデプロイは不要です。

---

## 提供ツール

特に断りがない限り、以下のツールは Web API 経由で動作し、設定されたトークンを `MCP_SLACK_USER_TOKEN` → `MCP_SLACK_TOKEN` → `MCP_SLACK_BOT_TOKEN` の優先順で使用します。

ツールは、対応するトランスポートの認証情報が設定されている場合にのみ登録されます。トークンのみを設定した環境では Webhook 系のツール（`post_slack_message`）は一覧に現れず、その逆も同様です。

Slackを変更するツール（`post_slack_message` / `post_slack_message_as_user` / `update_slack_message` / `delete_slack_message`）はすべて `confirm` ゲートを持ちます。実際にSlackへ書き込むのは `confirm=true` を指定したときだけで、`confirm` を省略/`false` にした場合は**Slackに一切触れず**、実際に送信される payload をプレビューとして返します。プレビュー専用の別ツールは意図的に用意していません（プレビューはツールを飛ばして到達できる別経路ではなく、既定の動作です）。

すべてのツールはMCPの tool annotations（`readOnlyHint` / `destructiveHint` / `idempotentHint` / `openWorldHint`）とタイトルを宣言しています。読み取り専用ツールの自動許可といったMCPクライアント側の権限制御が、ツール名の推測ではなく宣言に基づいて機能します。また `confirm` ゲートの手順（プレビュー → 人間の確認 → `confirm=true` で再実行）は、サーバーの instructions として initialize 時にクライアントへ一度だけ通知されます。

| ツール名 | 説明 |
| --- | --- |
| `post_slack_message` | `MCP_SLACK_WEBHOOK_URL` の Slack Incoming Webhook にメッセージを投稿 |
| `post_slack_message_as_user` | `chat.postMessage` でトークン所有者本人として投稿し、`channel_id` と `ts` を返す。プレビューはチャンネル名・メンション先・スレッド元メッセージを解決して表示 |
| `update_slack_message` | `chat.update` で投稿済みメッセージの内容を丸ごと置き換え。プレビューは更新前の内容と更新後の payload を並べて表示 |
| `delete_slack_message` | `chat.delete` でメッセージを削除（取り消し不可）。プレビューは削除対象メッセージの内容を表示 |
| `list_slack_channels` | `conversations.list` でワークスペース全体のチャンネル一覧を取得 |
| `list_joined_slack_channels` | `users.conversations` でトークン所有者が参加しているチャンネルのみを取得 |
| `get_slack_channel_info` | `conversations.info` で単一チャンネルの詳細情報を取得 |
| `get_slack_channel_history` | `conversations.history` でチャンネルのメッセージ履歴を取得 |
| `get_slack_thread_replies` | `conversations.replies` で指定メッセージのスレッド返信を取得 |
| `search_slack_messages` | `search.messages` でワークスペース全体を横断全文検索（ユーザートークン + `search:read` が必要） |
| `list_slack_users` | `users.list` でワークスペースメンバー一覧を取得 |
| `lookup_slack_user_by_email` | `users.lookupByEmail` でメールアドレスから単一ユーザーを検索 |
| `resolve_slack_user` | `name` または `email` から Slack ユーザーを一意に解決し、`<@U...>` 形式のmentionを返す |
| `get_slack_auth_info` | `auth.test` で現在のトークンの認証情報（team/user/bot_idなど）を確認。OAuthスコープ不要 |

各ツールの入力フィールド詳細・必要なOAuthスコープは [docs/tools.md](docs/tools.md) を参照してください。

---

## プロジェクトレイアウト (Project Layout)

```text
ap-mcp-slack/
├── main.go              # エントリーポイント
└── internal/
    ├── config/          # 環境変数ロード
    ├── app/             # DI コンテナ（SlackClient・設定の集約）
    ├── client/          # Slack Incoming Webhook / Web API クライアント
    ├── tools/           # MCPツール定義
    └── server/          # MCP stdio サーバー
```

---

## インストール / ビルド

`go install` で直接インストールできます。バイナリは `$(go env GOPATH)/bin/ap-mcp-slack` に置かれます。

```bash
go install github.com/shouni/ap-mcp-slack@latest
```

リポジトリを clone して手元でビルドする場合は以下です。

```bash
go build -o ./bin/ap-mcp-slack .
```

バージョンは MCP クライアントの初期化時にサーバー名とともに報告され、クライアント側のログでどのビルドと話しているかの識別に使われます。何もしなくてもビルド方法に応じて以下が自動で入ります。

| ビルド方法 | 報告されるバージョン |
| --- | --- |
| `go install ...@v1.2.3` | `v1.2.3` |
| リポジトリ内で `go build` | VCS由来の擬似バージョン（例: `v1.0.1-0.20260802190324-af39274744ae`。未コミットの変更があれば `+dirty`） |
| `go run` / `go test` / `-buildvcs=false` | `dev` |

擬似バージョンではなく `git describe` の短い表記にしたい場合は、ビルド時に埋め込めます。`-ldflags` を指定した場合はそちらが優先されます。

```bash
go build -ldflags "-X github.com/shouni/ap-mcp-slack/internal/server.Version=$(git describe --tags --always)" -o ./bin/ap-mcp-slack .
```

---

## MCPクライアントへの登録例

stdio transport に対応したMCPクライアントであれば、Codex 以外（Claude Code、Claude Desktop など）からも同じバイナリをそのまま起動できます。

以下の例では clone してビルドした `/path/to/ap-mcp-slack/bin/ap-mcp-slack` を指定しています。`go install` した場合は `$(go env GOPATH)/bin/ap-mcp-slack` に読み替えてください。

### Claude Code

```bash
claude mcp add ap-mcp-slack \
  -e MCP_SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXX/YYY/ZZZ \
  -e MCP_SLACK_USER_TOKEN=xoxp-... \
  -e MCP_SLACK_CHANNEL_ID=C0123456789 \
  -- /path/to/ap-mcp-slack/bin/ap-mcp-slack
```

### Codex

`~/.codex/config.toml` に登録します。

```toml
[mcp_servers.ap-mcp-slack]
command = "/path/to/ap-mcp-slack/bin/ap-mcp-slack"

[mcp_servers.ap-mcp-slack.env]
MCP_SLACK_WEBHOOK_URL = "https://hooks.slack.com/services/XXX/YYY/ZZZ"
MCP_SLACK_USER_TOKEN = "xoxp-..."
MCP_SLACK_CHANNEL_ID = "C0123456789"
```

開発中はビルドせずに `go run` で登録することもできます。

```toml
[mcp_servers.ap-mcp-slack]
command = "go"
args = ["run", "/path/to/ap-mcp-slack"]

[mcp_servers.ap-mcp-slack.env]
MCP_SLACK_WEBHOOK_URL = "https://hooks.slack.com/services/XXX/YYY/ZZZ"
MCP_SLACK_USER_TOKEN = "xoxp-..."
MCP_SLACK_CHANNEL_ID = "C0123456789"
```

---

## ローカル確認

stdio MCPサーバーなので、通常のHTTPサーバーのようにポートは開きません。手元で起動確認する場合は以下のように実行できますが、起動後はMCPクライアントからのJSON-RPC入力を待ちます。

```bash
export MCP_SLACK_WEBHOOK_URL="https://hooks.slack.com/services/XXX/YYY/ZZZ"
export MCP_SLACK_USER_TOKEN="xoxp-..."
export MCP_SLACK_CHANNEL_ID="C0123456789"
go run .
```

---

## 環境変数

| 環境変数 | 必須 | 説明 |
| --- | :---: | --- |
| `MCP_SLACK_WEBHOOK_URL` | いずれか必須 | Slack Incoming Webhook URL。Webhook投稿ツールを使う場合に必要。 |
| `MCP_SLACK_USER_TOKEN` | いずれか必須 | Slack Web API用のユーザートークン。トークン所有者本人としてSlackを操作する場合に指定（`search_slack_messages` はユーザートークン必須）。 |
| `MCP_SLACK_TOKEN` | いずれか必須 | Slack Web API用の汎用トークン。`MCP_SLACK_USER_TOKEN` が未指定の場合に利用。 |
| `MCP_SLACK_BOT_TOKEN` | いずれか必須 | Slack Web API用のBotトークン。上記2つが未指定の場合に利用。 |
| `MCP_SLACK_CHANNEL_ID` | 任意 | Web API 系ツールで `channel_id` を省略したときに使われるデフォルトチャンネルID。 |
| `MCP_SLACK_SOURCE_LABEL` | 任意 | `post_slack_message` / `post_slack_message_as_user` / `update_slack_message` の payload 末尾に付与する投稿元ラベル。Block Kitのcontextブロックとして自動付与されます。未設定時は `ap-mcp-slack (MCP) 経由`。 |

`MCP_SLACK_WEBHOOK_URL` とトークン（`MCP_SLACK_USER_TOKEN` / `MCP_SLACK_TOKEN` / `MCP_SLACK_BOT_TOKEN` のいずれか）は、少なくとも一方を設定してください。どちらも未設定の場合、登録できるツールが1つも無くなるため、サーバーは起動時にエラー終了します（正常に接続したうえでツールを1つも提示しない、という分かりにくい状態を避けるためです）。

必要な Slack トークンスコープは [docs/tools.md](docs/tools.md#必要な-slack-トークンスコープ) を参照してください。

---

## 主な依存関係 (Dependencies)

| パッケージ | 説明 |
| --- | --- |
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | MCP 公式 Go SDK（stdio トランスポート） |
| [slack-go/slack](https://github.com/slack-go/slack) | Slack Web API クライアント（chat.postMessage / chat.update / chat.delete / conversations.list / users.conversations / conversations.info / conversations.history / conversations.replies / search.messages / users.list / users.info / users.lookupByEmail / auth.test） |
| [shouni/go-http-kit](https://github.com/shouni/go-http-kit) | Webhook投稿用のHTTPクライアント（リトライ制御・SSRF/DNS Rebinding対策） |

---

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
