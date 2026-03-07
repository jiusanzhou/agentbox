package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"go.zoe.im/agentbox/internal/browser"
	davserver "go.zoe.im/agentbox/internal/bridge/webdav"
	"go.zoe.im/agentbox/internal/clipboard"
	"go.zoe.im/agentbox/internal/notify"
	"go.zoe.im/agentbox/internal/search"
	"go.zoe.im/agentbox/internal/shell"
	"go.zoe.im/agentbox/internal/tunnel"

	"go.zoe.im/x/cli"
)

var (
	clientServer    string
	clientToken     string
	clientRoots     string
	clientBrowser   bool
	clientChromeURL string
	clientShell     bool
	clientClipboard bool
	clientNotify    bool
	clientSearch    bool
	clientShellDirs string
	clientAll       bool
)

var clientCmd = cli.New(
	cli.Name("client"),
	cli.Short("Connect to ABox server and expose local capabilities"),
	cli.Run(runClient),
)

func init() {
	clientCmd.Flags().StringVarP(&clientServer, "server", "s", "", "ABox server URL (default: AGENTBOX_SERVER or http://localhost:8080)")
	clientCmd.Flags().StringVarP(&clientToken, "token", "t", "", "Auth token or API key")
	clientCmd.Flags().StringVarP(&clientRoots, "roots", "r", "", "Comma-separated directories to expose")
	clientCmd.Flags().BoolVar(&clientBrowser, "browser", false, "Enable browser capability")
	clientCmd.Flags().StringVar(&clientChromeURL, "chrome-url", "", "Chrome DevTools URL (default: auto-detect or launch)")
	clientCmd.Flags().BoolVar(&clientShell, "shell", false, "Enable shell command execution")
	clientCmd.Flags().BoolVar(&clientClipboard, "clipboard", false, "Enable clipboard access")
	clientCmd.Flags().BoolVar(&clientNotify, "notify", false, "Enable desktop notifications")
	clientCmd.Flags().BoolVar(&clientSearch, "search", false, "Enable file search")
	clientCmd.Flags().StringVar(&clientShellDirs, "shell-dirs", "", "Allowed directories for shell (comma-separated, required with --shell)")
	clientCmd.Flags().BoolVar(&clientAll, "all", false, "Enable all providers (--shell-dirs required for shell)")
}

func runClient(cmd *cli.Command, args ...string) {
	if clientServer == "" {
		clientServer = os.Getenv("AGENTBOX_SERVER")
	}
	if clientServer == "" {
		clientServer = serverAddr
	}
	if clientToken == "" {
		clientToken = os.Getenv("AGENTBOX_TOKEN")
	}
	if clientToken == "" {
		fmt.Fprintln(os.Stderr, "Error: --token or AGENTBOX_TOKEN is required")
		os.Exit(1)
	}

	logger := slog.Default()
	client := tunnel.NewClient(clientServer, clientToken, logger)

	// --all enables everything
	if clientAll {
		clientBrowser = true
		clientClipboard = true
		clientNotify = true
		clientSearch = true
		if clientShellDirs != "" {
			clientShell = true
		}
	}

	// Validate shell requires shell-dirs
	if clientShell && clientShellDirs == "" {
		fmt.Fprintln(os.Stderr, "Error: --shell-dirs is required when --shell is enabled")
		os.Exit(1)
	}

	// Register WebDAV provider if roots specified
	if clientRoots != "" {
		rootList := parseRootDirs(clientRoots)
		if len(rootList) > 0 {
			dav := davserver.NewHandler(rootList)
			client.AddProvider("webdav", dav)
			logger.Info("webdav provider registered", "roots", rootList)
		}
	}

	// Register browser provider if enabled
	if clientBrowser {
		cfg := browser.Config{
			RemoteURL: clientChromeURL,
			Headless:  false,
		}
		b, err := browser.New(cfg, logger)
		if err != nil {
			logger.Error("failed to start browser provider", "err", err)
		} else {
			client.AddProvider("browser", b.Handler())
			logger.Info("browser provider registered")
			defer b.Close()
		}
	}

	// Register shell provider if enabled
	if clientShell {
		dirs := parseRootDirs(clientShellDirs)
		sh := shell.New(shell.Config{
			AllowedDirs: dirs,
			Timeout:     30,
		})
		client.AddProvider("shell", sh.Handler())
		logger.Info("shell provider registered", "dirs", dirs)
	}

	// Register clipboard provider if enabled
	if clientClipboard {
		cb := clipboard.New()
		client.AddProvider("clipboard", cb.Handler())
		logger.Info("clipboard provider registered")
	}

	// Register notify provider if enabled
	if clientNotify {
		nt := notify.New()
		client.AddProvider("notify", nt.Handler())
		logger.Info("notify provider registered")
	}

	// Register search provider if enabled
	if clientSearch {
		var searchDirs []string
		if clientRoots != "" {
			searchDirs = parseRootDirs(clientRoots)
		}
		srch := search.New(search.Config{AllowedDirs: searchDirs})
		client.AddProvider("search", srch.Handler())
		logger.Info("search provider registered")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	fmt.Println()
	fmt.Printf("  \033[1mABox Client\033[0m\n")
	fmt.Printf("  Server: %s\n", clientServer)
	if clientRoots != "" {
		for i, r := range parseRootDirs(clientRoots) {
			fmt.Printf("    \033[33m/webdav/r%d/\033[0m -> %s\n", i, r)
		}
	}
	if clientBrowser {
		fmt.Printf("    \033[33m/browser/\033[0m -> Chrome CDP\n")
	}
	if clientShell {
		fmt.Printf("    \033[33m/shell/\033[0m -> Shell (%s)\n", clientShellDirs)
	}
	if clientClipboard {
		fmt.Printf("    \033[33m/clipboard/\033[0m -> System Clipboard\n")
	}
	if clientNotify {
		fmt.Printf("    \033[33m/notify/\033[0m -> Desktop Notifications\n")
	}
	if clientSearch {
		fmt.Printf("    \033[33m/search/\033[0m -> File Search\n")
	}
	fmt.Println()

	if err := client.Connect(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
