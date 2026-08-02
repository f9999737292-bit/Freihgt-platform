package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	command := flag.String("command", "", "preflight|snapshot|gate")
	flag.Parse()
	if *command != "" {
		_ = os.Setenv("OBSERVATION_COMMAND", *command)
	}
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "observation: config: %v\n", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := newHTTPClient(cfg.HTTPTimeout)
	var runErr error
	switch cfg.Command {
	case "preflight":
		runErr = runPreflight(ctx, cfg, client)
	case "snapshot":
		runErr = runSnapshot(ctx, cfg, client)
	case "gate":
		runErr = runGate(ctx, cfg, client)
	default:
		fmt.Fprintf(os.Stderr, "observation: unknown command %q\n", cfg.Command)
		os.Exit(2)
	}
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "observation: %v\n", runErr)
		os.Exit(1)
	}
}
