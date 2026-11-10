# AP MCP Slack

[![CI](https://github.com/shouni/ap-mcp-slack/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/ap-mcp-slack/actions/workflows/ci.yml)
[![Status](https://img.shields.io/badge/Status-Active-brightgreen)](#)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-MCP%20stdio-blue)](https://modelcontextprotocol.io/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

English | [日本語](README.ja.md)

## 🚀 About - a Slack MCP server over stdio, with no server to host

An MCP server that provides, as tools, posting, updating and deleting Slack messages, plus reading channels, history and threads, workspace-wide search, and user resolution. It supports two transports: Slack Incoming Webhook and the Slack Web API.

It is launched as a command by the MCP client and communicates over the stdio transport on stdin/stdout. No localhost HTTP server or Cloud Run deployment is needed.

---

## Tools

Unless noted otherwise, the tools below go through the Web API and use the configured token in the order of precedence `MCP_SLACK_USER_TOKEN` → `MCP_SLACK_TOKEN` → `MCP_SLACK_BOT_TOKEN`.

A tool is registered only when the credentials for its transport are configured. In an environment with only a token, the webhook tool (`post_slack_message`) does not appear in the list, and vice versa.

Every tool that changes Slack (`post_slack_message` / `post_slack_message_as_user` / `update_slack_message` / `delete_slack_message`) has a `confirm` gate. It actually writes to Slack only with `confirm=true`; with `confirm` omitted or `false` it **does not touch Slack at all** and returns the payload that would actually be sent as a preview. There is deliberately no separate preview-only tool (the preview is not a separate route that can be reached by skipping a tool; it is the default behaviour).

Every tool declares MCP tool annotations (`readOnlyHint` / `destructiveHint` / `idempotentHint` / `openWorldHint`) and a title. Permission controls on the MCP client side, such as auto-approving read-only tools, work from these declarations rather than from guessing at tool names. The `confirm` gate procedure (preview → human check → rerun with `confirm=true`) is sent to the client once, at initialize, as the server's instructions.

| Tool | Description |
| --- | --- |
| `post_slack_message` | Posts a message to the Slack Incoming Webhook at `MCP_SLACK_WEBHOOK_URL` |
| `post_slack_message_as_user` | Posts as the token owner through `chat.postMessage` and returns `channel_id` and `ts`. The preview resolves and shows the channel name, mentioned users and thread parent message |
| `update_slack_message` | Replaces the whole content of a posted message through `chat.update`. The preview shows the content before the update next to the updated payload |
| `delete_slack_message` | Deletes a message through `chat.delete` (irreversible). The preview shows the content of the message to be deleted |
| `list_slack_channels` | Lists the channels of the whole workspace through `conversations.list` |
| `list_joined_slack_channels` | Lists only the channels the token owner has joined, through `users.conversations` |
| `get_slack_channel_info` | Fetches the details of a single channel through `conversations.info` |
| `get_slack_channel_history` | Fetches a channel's message history through `conversations.history` |
| `get_slack_thread_replies` | Fetches the thread replies to a given message through `conversations.replies` |
| `search_slack_messages` | Full-text search across the whole workspace through `search.messages` (needs a user token + `search:read`) |
| `list_slack_users` | Lists workspace members through `users.list` |
| `lookup_slack_user_by_email` | Looks up a single user by email address through `users.lookupByEmail` |
| `resolve_slack_user` | Resolves a single Slack user from `name` or `email` and returns a mention in `<@U...>` form |
| `get_slack_auth_info` | Shows the current token's identity (team/user/bot_id, etc.) through `auth.test`. No OAuth scope needed |

For each tool's input fields and the OAuth scopes it needs, see [docs/tools.md](docs/tools.md).

---

## Project Layout

```text
ap-mcp-slack/
├── main.go              # Entry point
└── internal/
    ├── config/          # Environment variable loading
    ├── app/             # DI container (gathers SlackClient and settings)
    ├── client/          # Slack Incoming Webhook / Web API client
    ├── tools/           # MCP tool definitions
    └── server/          # MCP stdio server
```

---

## Install / Build

It can be installed directly with `go install`. The binary is placed at `$(go env GOPATH)/bin/ap-mcp-slack`.

```bash
go install github.com/shouni/ap-mcp-slack@latest
```

To clone the repository and build locally:

```bash
go build -o ./bin/ap-mcp-slack .
```

The version is reported along with the server name when the MCP client initializes, and is used in the client's logs to identify which build it is talking to. Without doing anything, the following is filled in automatically depending on how it was built.

| How it was built | Reported version |
| --- | --- |
| `go install ...@v1.2.3` | `v1.2.3` |
| `go build` inside the repository | A VCS-derived pseudo-version (e.g. `v1.0.1-0.20260802190324-af39274744ae`; `+dirty` if there are uncommitted changes) |
| `go run` / `go test` / `-buildvcs=false` | `dev` |

If you want the short `git describe` form instead of a pseudo-version, it can be embedded at build time. A value given through `-ldflags` takes precedence.

```bash
go build -ldflags "-X github.com/shouni/ap-mcp-slack/internal/server.Version=$(git describe --tags --always)" -o ./bin/ap-mcp-slack .
```

---

## Registering with an MCP client

Any MCP client that supports the stdio transport can launch the same binary as is, not only Codex (Claude Code, Claude Desktop and so on).

The examples below use `/path/to/ap-mcp-slack/bin/ap-mcp-slack`, cloned and built. If you used `go install`, read it as `$(go env GOPATH)/bin/ap-mcp-slack`.

### Claude Code

```bash
claude mcp add ap-mcp-slack \
  -e MCP_SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXX/YYY/ZZZ \
  -e MCP_SLACK_USER_TOKEN=xoxp-... \
  -e MCP_SLACK_CHANNEL_ID=C0123456789 \
  -- /path/to/ap-mcp-slack/bin/ap-mcp-slack
```

### Codex

Register it in `~/.codex/config.toml`.

```toml
[mcp_servers.ap-mcp-slack]
command = "/path/to/ap-mcp-slack/bin/ap-mcp-slack"

[mcp_servers.ap-mcp-slack.env]
MCP_SLACK_WEBHOOK_URL = "https://hooks.slack.com/services/XXX/YYY/ZZZ"
MCP_SLACK_USER_TOKEN = "xoxp-..."
MCP_SLACK_CHANNEL_ID = "C0123456789"
```

During development it can also be registered with `go run`, without building.

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

## Running locally

Since this is a stdio MCP server, it does not open a port like an ordinary HTTP server. You can start it by hand as below to check that it launches, but once started it waits for JSON-RPC input from an MCP client.

```bash
export MCP_SLACK_WEBHOOK_URL="https://hooks.slack.com/services/XXX/YYY/ZZZ"
export MCP_SLACK_USER_TOKEN="xoxp-..."
export MCP_SLACK_CHANNEL_ID="C0123456789"
go run .
```

---

## Environment variables

| Variable | Required | Description |
| --- | :---: | --- |
| `MCP_SLACK_WEBHOOK_URL` | One of these | The Slack Incoming Webhook URL. Needed to use the webhook posting tool. |
| `MCP_SLACK_USER_TOKEN` | One of these | A user token for the Slack Web API. Set it to act on Slack as the token owner (`search_slack_messages` requires a user token). |
| `MCP_SLACK_TOKEN` | One of these | A general-purpose token for the Slack Web API. Used when `MCP_SLACK_USER_TOKEN` is not set. |
| `MCP_SLACK_BOT_TOKEN` | One of these | A bot token for the Slack Web API. Used when neither of the two above is set. |
| `MCP_SLACK_CHANNEL_ID` | Optional | The default channel ID used when a Web API tool is called without `channel_id`. |
| `MCP_SLACK_SOURCE_LABEL` | Optional | The source label appended to the end of the payload of `post_slack_message` / `post_slack_message_as_user` / `update_slack_message`. It is added automatically as a Block Kit context block. Defaults to `ap-mcp-slack (MCP) 経由` ("via ap-mcp-slack (MCP)"). |

Set at least one of `MCP_SLACK_WEBHOOK_URL` and a token (any of `MCP_SLACK_USER_TOKEN` / `MCP_SLACK_TOKEN` / `MCP_SLACK_BOT_TOKEN`). With neither set there would be no tool to register, so the server exits with an error at startup (to avoid the confusing state of connecting normally and then offering no tools at all).

For the Slack token scopes required, see [docs/tools.md](docs/tools.md#required-slack-token-scopes).

---

## Dependencies

| Package | Description |
| --- | --- |
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | The official MCP Go SDK (stdio transport) |
| [slack-go/slack](https://github.com/slack-go/slack) | Slack Web API client (chat.postMessage / chat.update / chat.delete / conversations.list / users.conversations / conversations.info / conversations.history / conversations.replies / search.messages / users.list / users.info / users.lookupByEmail / auth.test) |
| [shouni/go-http-kit](https://github.com/shouni/go-http-kit) | HTTP client for webhook posting (retry control, SSRF/DNS rebinding protection) |

---

## 📜 License

This project is released under the [MIT License](https://opensource.org/licenses/MIT).
