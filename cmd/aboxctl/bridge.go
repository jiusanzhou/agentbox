package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go.zoe.im/agentbox/internal/bridge/mcpserver"
	davserver "go.zoe.im/agentbox/internal/bridge/webdav"

	"go.zoe.im/x/cli"
)

var (
	bridgeRoots      string
	bridgeMCPAddr    string
	bridgeWebDAVAddr string
)

var bridgeCmd = cli.New(
	cli.Name("bridge"),
	cli.Short("Start local data bridge (MCP + WebDAV) for agent containers"),
	cli.Run(runBridge),
)

func init() {
	bridgeCmd.Flags().StringVarP(&bridgeRoots, "roots", "r", "", "Comma-separated directories to expose (e.g. ~/Documents,~/projects)")
	bridgeCmd.Flags().StringVar(&bridgeMCPAddr, "mcp-addr", ":9800", "MCP HTTP server address")
	bridgeCmd.Flags().StringVarP(&bridgeWebDAVAddr, "webdav-addr", "w", ":9801", "WebDAV listen address")
}

func runBridge(cmd *cli.Command, args ...string) {
	if bridgeRoots == "" {
		fmt.Fprintln(os.Stderr, "Error: --roots is required")
		fmt.Fprintln(os.Stderr, "Example: aboxctl bridge --roots ~/Documents,~/projects")
		os.Exit(1)
	}

	rootList := parseRootDirs(bridgeRoots)
	logger := slog.Default()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	mcpSrv := mcpserver.New(rootList, logger)

	dav := davserver.New(davserver.Config{
		Addr:  bridgeWebDAVAddr,
		Roots: rootList,
	}, logger)
	go func() {
		if err := dav.Start(ctx); err != nil {
			logger.Error("webdav error", "err", err)
		}
	}()

	httpMCP := mcpserver.NewHTTPServer(mcpSrv, bridgeMCPAddr, logger)
	go func() {
		if err := httpMCP.Start(ctx); err != nil {
			logger.Error("mcp http error", "err", err)
		}
	}()

	fmt.Println()
	fmt.Printf("  \033[1mABox Bridge\033[0m\n")
	for i, r := range rootList {
		fmt.Printf("    \033[33m/r%d/\033[0m -> %s\n", i, r)
	}
	fmt.Println()
	fmt.Printf("  MCP:    http://localhost%s\n", bridgeMCPAddr)
	fmt.Printf("  WebDAV: http://localhost%s\n", bridgeWebDAVAddr)
	fmt.Println()

	<-sigCh
	cancel()
	logger.Info("shutting down")
}

func parseRootDirs(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "~/") {
			home, _ := os.UserHomeDir()
			p = home + p[1:]
		}
		result = append(result, p)
	}
	return result
}
