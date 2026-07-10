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

`main.go` → `internal/app` (DI container) → `internal/builder` (server assembly) → `internal/server` (stdio MCP server). `internal/client` composes two independent transports behind `SlackClient`: `webhookTransport` (Incoming Webhook, via go-http-kit, SSRF-protected, defined in `webhook.go`) and `webAPITransport` (token-authenticated Web API, via slack-go/slack, split across `webapi.go` for messages/channels and `users.go` for user lookup; `slack.go` holds just `SlackClient`/`SlackClientConfig` and shared helpers). `internal/tools` defines the MCP tools (message post/delete, channel listing, user listing/lookup) and delegates to `SlackClient`.
