package main

import (
	"embed"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

//go:embed index.html
var content embed.FS

func main() {
	port := flag.Int("port", 8088, "UI server port")
	targetAddr := flag.String("target", "localhost:8080", "Chat API address (host:port)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	raw, err := content.ReadFile("index.html")
	if err != nil {
		logger.Error("read embedded file", "error", err)
		os.Exit(1)
	}
	page := strings.ReplaceAll(string(raw), "{{TARGET}}", *targetAddr)

	targetURL, err := url.Parse(fmt.Sprintf("http://%s", *targetAddr))
	if err != nil {
		logger.Error("invalid target address", "error", err)
		os.Exit(1)
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})
	mux.Handle("/api/", proxy)
	mux.Handle("/admin/", proxy)

	addr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	logger.Info("ui dashboard", "addr", fmt.Sprintf("http://localhost%s", addr), "target", *targetAddr)
	if serveErr := srv.ListenAndServe(); serveErr != nil {
		logger.Error("listen", "error", serveErr)
		os.Exit(1)
	}
}
