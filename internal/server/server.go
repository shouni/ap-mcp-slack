// Package server builds and runs the MCP server.
package server

import (
	"context"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/app"
	"github.com/shouni/ap-mcp-slack/internal/tools"
)

// defaultVersion is what a build reports when neither -ldflags nor the module
// version identifies it.
const defaultVersion = "dev"

// Version is reported to the MCP client during initialization. Override it at build
// time so a client's logs identify which build it is talking to:
//
//	go build -ldflags "-X github.com/shouni/ap-mcp-slack/internal/server.Version=$(git describe --tags --always)"
//
// It must keep a constant initializer: -X only applies to a string variable that is
// uninitialized or initialized to a constant expression — computing it here would
// silently disable the flag above. Read it through resolveVersion rather than
// directly, so builds that -X never touches (`go install path@version` above all)
// still report the module version they were built at.
var Version = defaultVersion

// Server is an MCP server using stdio transport.
type Server struct {
	mcpServer *mcp.Server
}

// New creates a Server from the DI container and registers all tools.
func New(container *app.Container) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "ap-mcp-slack",
		Version: resolveVersion(Version, moduleVersion()),
	}, &mcp.ServerOptions{
		// Instructions reach the model once per session instead of once per tool
		// description; tools.ServerInstructions carries the cross-tool contract
		// (confirm gate, default channel, pagination).
		Instructions: tools.ServerInstructions,
	})

	tools.NewSlackTools(container.Slack).Register(mcpServer)

	return &Server{mcpServer: mcpServer}
}

// resolveVersion picks what to report, preferring an -ldflags value over the module
// version stamped into the binary. Both can be present on one build — a local
// `go build` inside this repo gets a VCS-derived pseudo-version — and -ldflags wins
// there because it is the deliberate one.
//
// What module holds, by build route:
//
//	go install path@v1.2.3                     "v1.2.3"
//	go build inside the repo                   pseudo-version, "+dirty" if uncommitted
//	go test / -buildvcs=false / outside a repo "(devel)"
//
// "(devel)" is discarded because it identifies a build less than "dev" already does.
func resolveVersion(injected, module string) string {
	if injected != defaultVersion {
		return injected
	}
	if module != "" && module != "(devel)" {
		return module
	}
	return defaultVersion
}

// moduleVersion returns the version this binary's main module was built at, or ""
// when it is unavailable (`go build` in a working tree, `go run`, tests).
func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

// Run starts the MCP server over stdin/stdout.
func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
