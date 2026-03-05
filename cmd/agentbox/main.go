package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.zoe.im/agentbox/internal/config"
	"go.zoe.im/agentbox/internal/service"

	"go.zoe.im/x/cli"
	"go.zoe.im/x/version"
)

var cmd = cli.New(
	cli.Name("agentbox"),
	cli.Short("Agent workflow execution platform"),
	cli.Description("Run natural language-described agent workflows in isolated sandbox environments."),
	version.NewOption(true),
	cli.GlobalConfig(
		config.Global(),
		cli.WithConfigName("agentbox"),
	),
	cli.Run(func(cmd *cli.Command, args ...string) {
		cfg := config.Global()

		svc, err := service.New(cfg)
		if err != nil {
			log.Fatalf("failed to init service: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh
			log.Println("shutting down...")
			cancel()
		}()

		if err := svc.Start(ctx); err != nil && err != context.Canceled {
			log.Fatal(err)
		}
	}),
)

func main() {
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
