package main

import (
	"log/slog"
	"os"

	"github.com/JspBack/end-to-end-chat/client"
	"github.com/JspBack/end-to-end-chat/config"
)

func main() {
	cl := client.New(*config.Parse(), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	cl.Listen()
}
