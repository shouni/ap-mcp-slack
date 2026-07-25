// Package server builds and runs the MCP server.
package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/app"
	"ap-mcp-slack/internal/tools"
)

// Version is reported to the MCP client during initialization. Override it at build
// time so a client's logs identify which build it is talking to:
//
//	go build -ldflags "-X ap-mcp-slack/internal/server.Version=$(git describe --tags --always)"
//
// It stays "dev" for plain `go build` and `go test`.
var Version = "dev"

// Server is an MCP server using stdio transport.
type Server struct {
	mcpServer *mcp.Server
}

// New creates a Server from the DI container and registers all tools.
func New(container *app.Container) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "ap-mcp-slack",
		Version: Version,
	}, nil)

	tools.NewSlackTools(container.Slack).Register(mcpServer)

	return &Server{mcpServer: mcpServer}
}

// Run starts the MCP server over stdin/stdout.
func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
