// Package server builds and runs the MCP server.
package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
	"ap-mcp-slack/internal/tools"
)

// Server is an MCP server using stdio transport.
type Server struct {
	mcpServer *mcp.Server
}

// New creates a Server and registers all tools.
func New(slackClient *client.SlackClient) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "ap-mcp-slack",
		Version: "dev",
	}, nil)

	tools.NewSlackTools(slackClient).Register(mcpServer)

	return &Server{mcpServer: mcpServer}
}

// Run starts the MCP server over stdin/stdout.
func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
