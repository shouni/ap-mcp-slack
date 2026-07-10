# AP MCP Slack

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Slack Incoming Webhook と Slack Web API で投稿・削除するための MCP サーバーです。

MCP クライアントからコマンドとして起動され、stdin/stdout の stdio transport で通信します。ローカルホストのHTTPサーバーやCloud Runデプロイは不要です。

## 提供ツール

| ツール名 | 説明 |
| --- | --- |
| `post_slack_message` | `MCP_SLACK_WEBHOOK_URL` の Slack Incoming Webhook にメッセージを投稿 |
| `post_slack_message_as_user` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `chat.postMessage` で投稿し、`channel_id` と `ts` を返す |
| `delete_slack_message` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `chat.delete` で投稿済みメッセージを削除 |
| `list_slack_channels` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `conversations.list` でワークスペース全体のチャンネル一覧を取得 |
| `list_joined_slack_channels` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `users.conversations` でトークン所有者が参加しているチャンネルのみを取得 |
| `list_slack_users` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `users.list` でワークスペースメンバー一覧を取得 |
| `lookup_slack_user_by_email` | `MCP_SLACK_USER_TOKEN` または `MCP_SLACK_TOKEN` を使って `users.lookupByEmail` でメールアドレスから単一ユーザーを検索 |
| `resolve_slack_user` | `name` または `email` から Slack ユーザーを一意に解決し、`<@U...>` 形式のmentionを返す |

`post_slack_message` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `text` | 必須 | Slackに投稿する本文。デフォルトでSlackのmrkdwnとして解釈されます。 |
| `blocks` | 任意 | Slack Block Kit の blocks 配列。 |
| `attachments` | 任意 | Slack attachments 配列。 |
| `thread_ts` | 任意 | スレッド返信にする場合の親メッセージts。Webhook側で利用可能な場合のみ有効です。 |
| `icon_emoji` | 任意 | 投稿者アイコンとして使うSlack絵文字名。例: `:robot_face:` |
| `unfurl_links` | 任意 | リンク展開の制御。 |
| `unfurl_media` | 任意 | メディア展開の制御。 |

`post_slack_message_as_user` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `text` | 必須 | Slackに投稿する本文。デフォルトでSlackのmrkdwnとして解釈されます。 |
| `channel_id` | 任意 | 投稿先チャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |
| `blocks` | 任意 | Slack Block Kit の blocks 配列。 |
| `attachments` | 任意 | Slack attachments 配列。 |
| `thread_ts` | 任意 | スレッド返信にする場合の親メッセージts。 |
| `icon_emoji` | 任意 | 投稿者アイコンとして使うSlack絵文字名。例: `:robot_face:` |
| `unfurl_links` | 任意 | リンク展開の制御。 |
| `unfurl_media` | 任意 | メディア展開の制御。 |

`delete_slack_message` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `ts` | 必須 | 削除対象メッセージのts。`post_slack_message_as_user` の戻り値を利用できます。 |
| `channel_id` | 任意 | 削除対象のチャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |

`list_slack_channels` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `types` | 任意 | 取得する会話種別。`public_channel`, `private_channel`, `mpim`, `im` を指定できます。省略時は Slack API のデフォルト `public_channel` です。 |
| `exclude_archived` | 任意 | `true` の場合、アーカイブ済みチャンネルを除外します。 |
| `limit` | 任意 | 最大取得件数。省略時は `200`、最大 `1000` です。 |
| `cursor` | 任意 | 続きから取得する場合の Slack pagination cursor。 |
| `team_id` | 任意 | Enterprise Grid の org-level token で対象ワークスペースを指定する場合に使います。 |
| `sort` | 任意 | 取得した結果に適用する返却前の並び順。`none`, `name_asc`, `name_desc`, `created_asc`, `created_desc` を指定できます。省略時は `name_asc` です。 |

Slack API の `conversations.list` には並び順を指定する引数がないため、`sort` は MCP サーバーが取得した結果にローカルで適用します。

`list_slack_channels` で `private_channel` を含めて取得するには、トークンに `groups:read` スコープが必要です（`public_channel` のみなら `channels:read` で足ります）。

`list_joined_slack_channels` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `types` | 任意 | 取得する会話種別。`public_channel`, `private_channel`, `mpim`, `im` を指定できます。省略時は Slack API のデフォルト `public_channel` です。 |
| `exclude_archived` | 任意 | `true` の場合、アーカイブ済みチャンネルを除外します。 |
| `limit` | 任意 | 最大取得件数。省略時は `200`、最大 `1000` です。 |
| `cursor` | 任意 | 続きから取得する場合の Slack pagination cursor。 |
| `team_id` | 任意 | Enterprise Grid の org-level token で対象ワークスペースを指定する場合に使います。 |
| `sort` | 任意 | 取得した結果に適用する返却前の並び順。`none`, `name_asc`, `name_desc`, `created_asc`, `created_desc` を指定できます。省略時は `name_asc` です。 |

`list_slack_channels` がワークスペース全体を返すのに対し、`list_joined_slack_channels` は `users.conversations` を使うため、サーバー側でトークン所有者のメンバーシップに絞り込まれた結果のみが返ります。`MCP_SLACK_USER_TOKEN`（ユーザートークン）を設定していればそのユーザー本人が参加しているチャンネル、ボットトークンのみの場合はそのボットが参加しているチャンネルが対象です。

`list_slack_users` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `query` | 任意 | `name` / `real_name` / `profile.display_name` / `email` に対する部分一致検索（大文字小文字を区別しません）。 |
| `limit` | 任意 | 最大取得件数。省略時は `200`、最大 `1000` です。 |
| `cursor` | 任意 | 続きから取得する場合の Slack pagination cursor。 |
| `team_id` | 任意 | Enterprise Grid の org-level token で対象ワークスペースを指定する場合に使います。 |
| `include_deleted` | 任意 | `true` の場合、deactivate済み(deleted)ユーザーも含めます。省略時は除外されます。 |

`lookup_slack_user_by_email` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `email` | 必須 | 検索対象のメールアドレス。 |

`resolve_slack_user` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `name` | 任意 | 検索対象のユーザー名・real name・display nameのいずれか。`email` が指定された場合は無視されます。 |
| `email` | 任意 | 検索対象のメールアドレス。指定された場合は `name` より優先され、`users.lookupByEmail` で解決します。 |
| `team_id` | 任意 | Enterprise Grid の org-level token で対象ワークスペースを指定する場合に使います。`name` での検索時のみ利用します。 |

`email` が指定されない場合、`name` はまず `users.list` から取得したユーザーの `name` / `real_name` / `display_name` との完全一致（大文字小文字を区別しない）を探します。完全一致が1件もない場合は部分一致にフォールバックします。一致が1件のときのみ `status: "found"` として `user` と `<@U...>` 形式の `mention` を返します。0件なら `status: "not_found"`、複数件なら `status: "ambiguous"` として `candidates` に候補一覧を返し、誤送信を避けるため自動選択はしません。

## プロジェクトレイアウト (Project Layout)

```text
ap-mcp-slack/
├── main.go              # エントリーポイント
└── internal/
    ├── config/          # 環境変数ロード
    ├── app/             # DI コンテナ（SlackClient・設定の集約）
    ├── builder/         # コンテナから Server を組み立てる DI
    ├── client/          # Slack Incoming Webhook / Web API クライアント
    │   ├── slack.go     # SlackClient / SlackClientConfig とコンストラクタ
    │   ├── webhook.go   # webhookTransport（Incoming Webhook投稿）
    │   ├── webapi.go    # webAPITransport（メッセージ投稿/削除・チャンネル一覧）
    │   └── users.go     # ユーザー一覧・検索・解決
    ├── tools/           # MCP ツール定義
    │   └── slack.go     # 8ツールの入出力定義とハンドラ
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
| `MCP_SLACK_SOURCE_LABEL` | 任意 | `post_slack_message_as_user` の投稿末尾に付与する投稿元ラベル。ユーザートークンでの投稿はSlack上で本人の投稿と見分けがつかないため、Block Kitのcontextブロックとして自動付与されます。未設定時は `ap-mcp-slack (MCP) 経由`。 |

MCPサーバーの起動自体には Slack の環境変数は必須ではありません。未設定の機能を呼び出した場合は、各ツールが `webhook URL is required` や `token is required` を返します。

## 必要な Slack トークンスコープ

| スコープ | 用途 |
| --- | --- |
| `chat:write` | `post_slack_message_as_user` / `delete_slack_message` |
| `channels:read` | `list_slack_channels` / `list_joined_slack_channels`（`public_channel`） |
| `groups:read` | `list_slack_channels` / `list_joined_slack_channels` で `private_channel` を含める場合 |
| `users:read` | `list_slack_users` / `resolve_slack_user`（name検索） |
| `users:read.email` | `lookup_slack_user_by_email` / `resolve_slack_user`（email検索） |

## 主な依存関係 (Dependencies)

| パッケージ | 説明 |
| --- | --- |
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | MCP 公式 Go SDK（stdio トランスポート） |
| [slack-go/slack](https://github.com/slack-go/slack) | Slack Web API クライアント（chat.postMessage / chat.delete / conversations.list / users.conversations / users.list / users.lookupByEmail） |
| [shouni/go-http-kit](https://github.com/shouni/go-http-kit) | Webhook投稿用のHTTPクライアント（リトライ制御・SSRF/DNS Rebinding対策） |

## ライセンス

MIT License
