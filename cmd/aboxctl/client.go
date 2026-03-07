package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	davserver "go.zoe.im/agentbox/internal/bridge/webdav"
	"go.zoe.im/agentbox/internal/tunnel"

	"go.zoe.im/x/cli"
)

var (
	clientServer string
	clientToken  string
	clientRoots  string
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

	// Register WebDAV provider if roots specified
	if clientRoots != "" {
		rootList := parseRootDirs(clientRoots)
		if len(rootList) > 0 {
			dav := davserver.NewHandler(rootList)
			client.AddProvider("webdav", dav)
			logger.Info("webdav provider registered", "roots", rootList)
		}
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
	fmt.Println()

	if err := client.Connect(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
