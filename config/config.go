package config

import (
	"flag"
	"os"
	"time"
)

type Config struct {
	ClientName     string
	DB             string
	KeyFile        string
	Port           int
	Timeout        time.Duration
	RateLimit      int
	RateWindow     time.Duration
	MaxMessageSize int64
}

const (
	DefaultStore            = "data/chat.db"
	DefaultClientName       = "default"
	DefaultKeyFile          = "data/.generated_key"
	DefaultPort             = 8080
	DefaultTimeout          = 15 * time.Second
	DefaultRateLimit        = 100
	DefaultRateWindow       = time.Minute
	MaxMessageSize    int64 = 1 << 20
)

func Parse() *Config {
	c := &Config{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&c.ClientName, "client", DefaultClientName, "client name to use")
	fs.StringVar(&c.DB, "db", DefaultStore, "database name")
	fs.StringVar(&c.KeyFile, "k", DefaultKeyFile, "key file to use")
	fs.IntVar(&c.Port, "p", DefaultPort, "port to listen on")
	fs.DurationVar(&c.Timeout, "t", DefaultTimeout, "timeout for operations")
	fs.IntVar(&c.RateLimit, "rate-limit", DefaultRateLimit, "HTTP requests per window per IP")
	fs.DurationVar(&c.RateWindow, "rate-window", DefaultRateWindow, "HTTP rate limiter window duration")
	fs.Int64Var(&c.MaxMessageSize, "max-msg-size", MaxMessageSize, "maximum message size in bytes")
	fs.Usage = func() {
		println("Usage:", fs.Name(), "[options]")
		println("Options:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		fs.Usage()
		os.Exit(1)
	}
	return c
}
