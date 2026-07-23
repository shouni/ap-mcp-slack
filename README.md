# AP MCP Slack

[![CI](https://github.com/shouni/ap-mcp-slack/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/ap-mcp-slack/actions/workflows/ci.yml)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/ap-mcp-slack)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/ap-mcp-slack)](https://github.com/shouni/ap-mcp-slack/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Slack Incoming Webhook と Slack Web API で投稿・削除するための MCP サーバーです。

MCP クライアントからコマンドとして起動され、stdin/stdout の stdio transport で通信します。ローカルホストのHTTPサーバーやCloud Runデプロイは不要です。

## 提供ツール

| ツール名 | 説明 |
| --- | --- |
| `preview_slack_message` | `post_slack_message` で送信される Incoming Webhook payload を投稿せずに確認 |
| `post_slack_message` | `confirm=true` の場合のみ `MCP_SLACK_WEBHOOK_URL` の Slack Incoming Webhook にメッセージを投稿。`confirm` を省略/falseにした場合は投稿せずプレビューのみ返す |
| `preview_slack_message_as_user` | `post_slack_message_as_user` で送信される `chat.postMessage` payload を投稿せずに確認 |
| `post_slack_message_as_user` | `confirm=true` の場合のみ `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `chat.postMessage` で投稿し、`channel_id` と `ts` を返す。`confirm` を省略/falseにした場合は投稿せず、チャンネル名・メンション先・スレッド元メッセージを解決したプレビューのみ返す |
| `update_slack_message` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `chat.update` で投稿済みメッセージの内容を更新 |
| `delete_slack_message` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `chat.delete` で投稿済みメッセージを削除 |
| `list_slack_channels` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `conversations.list` でワークスペース全体のチャンネル一覧を取得 |
| `list_joined_slack_channels` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `users.conversations` でトークン所有者が参加しているチャンネルのみを取得 |
| `get_slack_channel_info` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `conversations.info` で単一チャンネルの詳細情報を取得 |
| `get_slack_channel_history` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `conversations.history` でチャンネルのメッセージ履歴を取得 |
| `get_slack_thread_replies` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `conversations.replies` で指定メッセージのスレッド返信を取得 |
| `list_slack_users` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `users.list` でワークスペースメンバー一覧を取得 |
| `lookup_slack_user_by_email` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `users.lookupByEmail` でメールアドレスから単一ユーザーを検索 |
| `resolve_slack_user` | `name` または `email` から Slack ユーザーを一意に解決し、`<@U...>` 形式のmentionを返す |
| `get_slack_auth_info` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `auth.test` で現在のトークンの認証情報（team/user/bot_idなど）を確認。OAuthスコープ不要 |

各ツールの入力フィールド詳細・必要なOAuthスコープは [docs/tools.md](docs/tools.md) を参照してください。

## プロジェクトレイアウト (Project Layout)

```text
ap-mcp-slack/
├── main.go              # エントリーポイント
└── internal/
    ├── config/          # 環境変数ロード
    ├── app/             # DI コンテナ（SlackClient・設定の集約）
    ├── builder/         # コンテナから Server を組み立てる DI
    ├── client/          # Slack Incoming Webhook / Web API クライアント
    ├── tools/           # MCP ツール定義
    └── server/          # MCP stdio サーバー
```

## ビルド

```bash
go build -o ./bin/ap-mcp-slack .
```

## MCPクライアントへの登録例

stdio transport に対応した MCP クライアントであれば、Codex 以外（Claude Code、Claude Desktop など）からも同じバイナリをそのまま起動できます。

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

## ローカル確認

stdio MCPサーバーなので、通常のHTTPサーバーのようにポートは開きません。手元で起動確認する場合は以下のように実行できますが、起動後はMCPクライアントからのJSON-RPC入力を待ちます。

```bash
export MCP_SLACK_WEBHOOK_URL="https://hooks.slack.com/services/XXX/YYY/ZZZ"
export MCP_SLACK_USER_TOKEN="xoxp-..."
export MCP_SLACK_CHANNEL_ID="C0123456789"
go run .
```

## 環境変数

| 環境変数 | 必須 | 説明 |
| --- | :---: | --- |
| `MCP_SLACK_WEBHOOK_URL` | ツール利用時 | Slack Incoming Webhook URL。Webhook投稿ツールを使う場合に必要。 |
| `MCP_SLACK_USER_TOKEN` | ツール利用時 | Slack Web API用のユーザートークン。本人として投稿・削除する場合に指定。 |
| `MCP_SLACK_TOKEN` | ツール利用時 | Slack Web API用の汎用トークン。`MCP_SLACK_USER_TOKEN` が未指定の場合に利用。 |
| `MCP_SLACK_BOT_TOKEN` | ツール利用時 | Slack Web API用のBotトークン。上記2つが未指定の場合に利用。 |
| `MCP_SLACK_CHANNEL_ID` | 任意 | Web API投稿・削除のデフォルトチャンネルID。ツール入力の `channel_id` で上書き可能。 |
| `MCP_SLACK_SOURCE_LABEL` | 任意 | `preview_slack_message` / `post_slack_message` / `preview_slack_message_as_user` / `post_slack_message_as_user` の payload 末尾に付与する投稿元ラベル。Block Kitのcontextブロックとして自動付与されます。未設定時は `ap-mcp-slack (MCP) 経由`。 |

MCPサーバーの起動自体には Slack の環境変数は必須ではありません。未設定の機能を呼び出した場合は、各ツールが `webhook URL is required` や `token is required` を返します。

必要な Slack トークンスコープは [docs/tools.md](docs/tools.md#必要な-slack-トークンスコープ) を参照してください。

## 主な依存関係 (Dependencies)

| パッケージ | 説明 |
| --- | --- |
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | MCP 公式 Go SDK（stdio トランスポート） |
| [slack-go/slack](https://github.com/slack-go/slack) | Slack Web API クライアント（chat.postMessage / chat.update / chat.delete / conversations.list / users.conversations / conversations.info / conversations.history / conversations.replies / users.list / users.lookupByEmail / auth.test） |
| [shouni/go-http-kit](https://github.com/shouni/go-http-kit) | Webhook投稿用のHTTPクライアント（リトライ制御・SSRF/DNS Rebinding対策） |

## ライセンス

MIT License
