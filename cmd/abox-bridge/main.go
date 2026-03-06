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
	"go.zoe.im/x/version"
)

var (
	roots      string
	mcpAddr    string
	webdavAddr string
	mode       string
)

var cmd = cli.New(
	cli.Name("abox-bridge"),
	cli.Short("ABox local data bridge - MCP + WebDAV server"),
	version.NewOption(true),
	cli.Run(run),
)

func init() {
	cmd.Flags().StringVarP(&roots, "roots", "r", "", "Comma-separated directories to expose (e.g. ~/Documents,~/projects)")
	cmd.Flags().StringVar(&mcpAddr, "mcp-addr", ":9800", "MCP HTTP server address")
	cmd.Flags().StringVarP(&webdavAddr, "webdav-addr", "w", ":9801", "WebDAV listen address")
	cmd.Flags().StringVarP(&mode, "mode", "m", "all", "Server mode: all, mcp, webdav")
}

func run(cmd *cli.Command, args ...string) {
	if roots == "" {
		fmt.Fprintln(os.Stderr, "Error: --roots is required")
		fmt.Fprintln(os.Stderr, "Example: abox-bridge --roots ~/Documents,~/projects")
		os.Exit(1)
	}

	rootList := parseRoots(roots)
	logger := slog.Default()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	mcpSrv := mcpserver.New(rootList, logger)

	if mode == "all" || mode == "webdav" {
		dav := davserver.New(davserver.Config{
			Addr:  webdavAddr,
			Roots: rootList,
		}, logger)

		go func() {
			if err := dav.Start(ctx); err != nil {
				logger.Error("webdav error", "err", err)
			}
		}()
	}

	if mode == "all" || mode == "mcp" {
		httpMCP := mcpserver.NewHTTPServer(mcpSrv, mcpAddr, logger)
		go func() {
			if err := httpMCP.Start(ctx); err != nil {
				logger.Error("mcp http error", "err", err)
			}
		}()
	}

	fmt.Println()
	fmt.Println("  \033[1mABox Bridge\033[0m")
	for i, r := range rootList {
		fmt.Printf("    \033[33m/r%d/\033[0m -> %s\n", i, r)
	}
	fmt.Println()
	if mode == "all" || mode == "mcp" {
		fmt.Printf("  MCP:    http://localhost%s\n", mcpAddr)
	}
	if mode == "all" || mode == "webdav" {
		fmt.Printf("  WebDAV: http://localhost%s\n", webdavAddr)
	}
	fmt.Println()

	<-sigCh
	cancel()
	logger.Info("shutting down")
}

func parseRoots(s string) []string {
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

func main() {
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
