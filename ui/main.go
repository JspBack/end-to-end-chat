package main

import (
	"embed"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

//go:embed index.html
var content embed.FS

func main() {
	port := flag.Int("p", 8088, "UI server port")
	targetAddr := flag.String("t", "127.0.0.1:8080", "Chat API address (host:port)")
	flag.Parse()

	raw, err := content.ReadFile("index.html")
	if err != nil {
		panic(err)
	}
	page := strings.ReplaceAll(string(raw), "{{TARGET}}", *targetAddr)

	targetURL, err := url.Parse(fmt.Sprintf("http://%s", *targetAddr))
	if err != nil {
		panic(err)
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
	fmt.Printf("ui at http://localhost%s (target %s)\n", addr, *targetAddr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
		panic(serveErr)
	}
}
