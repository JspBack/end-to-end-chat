package main

import (
	"log/slog"
	"os"

	"github.com/JspBack/end-to-end-chat/client"
	"github.com/JspBack/end-to-end-chat/config"
)

func main() {
	cfg := config.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client.New(cfg.ClientName, cfg.DB, cfg.KeyFile)
	logger.Info("starting client", "client", cfg.ClientName, "peer", cfg.Peer, "store", cfg.DB)
}
