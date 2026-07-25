# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

AP MCP Slack is a stdio-transport MCP (Model Context Protocol) server that exposes Slack Incoming Webhook and Slack Web API posting/deletion/listing as MCP tools. It is spawned by an MCP client (e.g. Codex) as a subprocess and communicates over stdin/stdout — there is no HTTP server or Cloud Run deployment involved.

Key dependencies (see `go.mod` / README.md):
- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) — official Go SDK for MCP
- [slack-go/slack](https://github.com/slack-go/slack) — Slack Web API client (chat.postMessage / chat.delete / conversations.list / users.conversations / users.list / users.lookupByEmail)
- [shouni/go-http-kit](https://github.com/shouni/go-http-kit) — HTTP client helpers (retry, SSRF/DNS-rebinding protection) used for the Incoming Webhook path

## Module

- Module path: `ap-mcp-slack`
- Go version: 1.26 (see `go.mod`)

## Architecture

`main.go` → `internal/app` (DI container) → `internal/builder` (server assembly) → `internal/server` (stdio MCP server). `internal/client` composes two independent transports behind `SlackClient`: `webhookTransport` (Incoming Webhook, via go-http-kit, SSRF-protected, defined in `webhook.go`) and `webAPITransport` (token-authenticated Web API, via slack-go/slack, defined in `webapi.go` and split across `webapi_messages.go` for post/update/delete, `webapi_channels.go` for channel listing/paging/sorting, `webapi_history.go` for conversation history/replies/text extraction, and `users.go` for user lookup; `slack.go` holds just `SlackClient`/`SlackClientConfig` and shared helpers). `internal/tools` defines the MCP tools (message post/update/delete, channel listing, user listing/lookup) and delegates to `SlackClient`.

Two invariants hold across the tool layer:

- **Registration is gated per transport.** `tools.Register` only advertises the webhook tools when a webhook URL is configured, and the Web API tools when a token is. A model picks from the advertised list, so a tool it has no credentials for must not be visible. `config.Load` fails when neither is set, since that would leave the server advertising nothing.
- **Every mutating tool is confirm-gated.** `post_slack_message`, `post_slack_message_as_user`, `update_slack_message`, and `delete_slack_message` all build and return a preview unconditionally, and only touch Slack when `confirm=true`. Update/delete previews resolve the target message so the caller sees what is about to be overwritten or destroyed. Keep any new mutating tool on this pattern.
