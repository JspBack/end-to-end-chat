package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/JspBack/end-to-end-chat/client"
	"github.com/JspBack/end-to-end-chat/config"
)

func main() {
	cfg := config.Parse()
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	cl := client.New(*cfg, l)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	if cfg.PeerAddr != "" {
		go func() { _ = cl.ConnectToPeer(ctx) }()
	}

	go func() {
		<-sig
		l.Info("shutting down...")
		cancel()
		cl.Shutdown()
	}()

	cl.Listen()
}
