// Package builder assembles the MCP server from the DI container.
package builder

import (
	"ap-mcp-slack/internal/app"
	"ap-mcp-slack/internal/server"
)

// BuildServer assembles the MCP server from the DI container.
func BuildServer(container *app.Container) (*server.Server, error) {
	return server.New(container), nil
}
