# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

AP MCP Slack is a stdio-transport MCP (Model Context Protocol) server that exposes Slack Incoming Webhook and Slack Web API posting/deletion/listing as MCP tools. It is spawned by an MCP client (e.g. Codex) as a subprocess and communicates over stdin/stdout — there is no HTTP server or Cloud Run deployment involved.

Key dependencies (see `go.mod` / README.md):
- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) — official Go SDK for MCP
- [slack-go/slack](https://github.com/slack-go/slack) — Slack Web API client (chat.postMessage / chat.delete / conversations.list / users.conversations / users.list / users.lookupByEmail)
- [shouni/go-http-kit](https://github.com/shouni/go-http-kit) — HTTP client helpers (retry, SSRF/DNS-rebinding protection) used for the Incoming Webhook path

## Module

- Module path: `github.com/shouni/ap-mcp-slack`
- Go version: 1.26 (see `go.mod`)

## Architecture

`main.go` → `internal/app` (DI container) → `internal/server` (stdio MCP server). `internal/client` composes two independent transports behind `SlackClient`: `webhookTransport` (Incoming Webhook, via go-http-kit, SSRF-protected, defined in `webhook.go`) and `webAPITransport` (token-authenticated Web API, via slack-go/slack, defined in `webapi.go` and split across `webapi_messages.go` for post/update/delete, `webapi_channels.go` for channel listing/paging/sorting, `webapi_history.go` for conversation history/replies/text extraction, `users.go` for user lookup, and `auth.go` for auth.test; `slack.go` holds `SlackClient`/`SlackClientConfig` plus the shared paginator and source-label helper). `internal/tools` defines the MCP tools (message post/update/delete, channel listing/info/history, user listing/lookup/resolution, auth info) and delegates to `SlackClient`.

Four invariants hold across these layers:

- **Registration is gated per transport.** `tools.Register` only advertises the webhook tools when a webhook URL is configured, and the Web API tools when a token is. A model picks from the advertised list, so a tool it has no credentials for must not be visible. `config.Load` fails when neither is set, since that would leave the server advertising nothing.
- **Every mutating tool is confirm-gated, and previewing is that gate rather than its own tool.** `post_slack_message`, `post_slack_message_as_user`, `update_slack_message`, and `delete_slack_message` all build and return a preview unconditionally, and only touch Slack when `confirm=true`. Update/delete previews resolve both the target message and the resolved replacement payload, so the caller sees what is about to be overwritten and what with. Do not add a `preview_*` tool: it would be a second route to the same payload, and which route a model picks would then decide whether the gate applies.
- **A preview is byte-for-byte what gets sent.** The `Preview*` methods on the client are the only place the source-label footer is applied, and every send path (`PostMessage`, `PostWebAPIMessage`, `UpdateWebAPIMessage`) routes through the matching `Preview*` first. `buildContentOptions` doubles as the payload validator so a preview rejects exactly what a send would. A second footer implementation on a send path would drift from the one a human approved — `TestSentBlocksMatchPreviewedBlocks` pins this.
- **Paginated listings never drop what they fetched, and are always bounded.** `collectPages` in `slack.go` backs every listing (channels, joined channels, users). Slack's cursor resumes after a whole page, so a page's items are all kept even past `limit`; truncating a page while still returning its cursor would make the dropped items permanently unreachable. `maxListPages` caps requests per call, because a filter that matches little would otherwise walk a whole workspace — the per-request timeout bounds each request, not their number. Route any new listing through it rather than hand-rolling a loop.

Failing to *read* something is generally not a failure of the operation, and "we gave up looking" is never reported as "it isn't there". `chat.update`/`chat.delete` need neither a history scope nor `channels:read`, so `resolveTarget` degrades an unreadable message body or channel name into a note instead of an error; `GetMessage` distinguishes a truncated thread walk from a genuine miss (`targetNoteSearchTruncated` vs `targetNoteNotFound`) so a caller is not sent to re-check a `ts` that is fine; `ResolveUser` reports `search_truncated` rather than passing off a capped scan as a definitive `not_found`. Preserve both distinctions when adding lookups: only an operation with no determinable target is an error.

Tool descriptions in `tools/slack.go` say what a tool does and how to choose between neighbours, and never name the environment variable behind the token — a tool is only advertised once its credentials exist, so that detail is something the model cannot act on, paid for in context on every request. Credentials and OAuth scopes belong in `docs/tools.md`.
