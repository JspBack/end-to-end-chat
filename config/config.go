package config

import (
	"flag"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode"
)

var Version = "placeholder"

const maxNameLen = 64

const (
	DefaultClientName       = "default"
	DefaultPort             = 8080
	DefaultTimeout          = 15 * time.Second
	DefaultRateLimit        = 100
	DefaultRateWindow       = time.Minute
	MaxMessageSize    int64 = 50 << 20
	DefaultPingWindow       = 5 * time.Second
)

type Config struct {
	ClientName     string
	LogLevel       slog.Level
	Port           int
	Timeout        time.Duration
	RateLimit      int
	RateWindow     time.Duration
	MaxMessageSize int64
	PingWindow     time.Duration
	CertFile       string
	KeyFileTLS     string

	PeerAddr  string
	WriteMode bool
}

func sanitizeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
	if len(s) > maxNameLen {
		s = s[:maxNameLen]
	}
	return s
}

func (c *Config) UseTLS() bool {
	return c.CertFile != "" && c.KeyFileTLS != ""
}

func Parse() *Config {
	c := &Config{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&c.ClientName, "client", DefaultClientName, "client name to use")
	fs.IntVar(&c.Port, "p", DefaultPort, "port to listen on")
	fs.DurationVar(&c.Timeout, "t", DefaultTimeout, "timeout for operations")
	fs.IntVar(&c.RateLimit, "rate-limit", DefaultRateLimit, "HTTP requests per window per IP")
	fs.DurationVar(&c.RateWindow, "rate-window", DefaultRateWindow, "HTTP rate limiter window duration")
	fs.Int64Var(&c.MaxMessageSize, "max-msg-size", MaxMessageSize, "maximum message size in bytes")
	fs.DurationVar(&c.PingWindow, "ping-window", DefaultPingWindow, "ping window duration")
	fs.TextVar(&c.LogLevel, "l", slog.LevelInfo, "log level (debug, info, warn, error)")
	fs.StringVar(&c.PeerAddr, "addr", "", "address of the remote server to connect to as a peer")
	fs.BoolVar(&c.WriteMode, "w", false, "write mode: read stdin and broadcast messages to connected peers")
	fs.StringVar(&c.CertFile, "cert", "", "TLS certificate file path")
	fs.StringVar(&c.KeyFileTLS, "key", "", "TLS private key file path")
	verInfo := fs.Bool("v", false, "show version")
	fs.Usage = func() {
		println("Usage:", fs.Name(), "[options]")
		println("Options:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		fs.Usage()
		os.Exit(1)
	}
	if *verInfo {
		println(Version)
		os.Exit(0)
	}
	c.ClientName = sanitizeName(c.ClientName)
	return c
}
