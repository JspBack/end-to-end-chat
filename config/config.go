package config

import (
	"flag"
)

type Config struct {
	Peer       string
	ClientName string
	DB         string
	KeyFile    string
}

const (
	DefaultStore      = "data/chat.db"
	DefaultClientName = "default"
	DefaultKeyFile    = "data/.generated_key"
)

func Parse() *Config {
	c := &Config{}
	flag.StringVar(&c.ClientName, "client", DefaultClientName, "client name to use")
	flag.StringVar(&c.DB, "db", DefaultStore, "database name")
	flag.StringVar(&c.KeyFile, "k", DefaultKeyFile, "key file to use")
	flag.Parse()
	return c
}
