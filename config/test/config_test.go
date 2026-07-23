package config_test

import (
	"testing"
	"time"

	"github.com/JspBack/end-to-end-chat/config"
)

func TestUseTLS(t *testing.T) {
	c := &config.Config{}
	if c.UseTLS() {
		t.Error("UseTLS should be false when both are empty")
	}

	c = &config.Config{CertFile: "/path/cert.pem"}
	if c.UseTLS() {
		t.Error("UseTLS should be false when only cert is set")
	}

	c = &config.Config{KeyFileTLS: "/path/key.pem"}
	if c.UseTLS() {
		t.Error("UseTLS should be false when only key is set")
	}

	c = &config.Config{CertFile: "/path/cert.pem", KeyFileTLS: "/path/key.pem"}
	if !c.UseTLS() {
		t.Error("UseTLS should be true when both cert and key are set")
	}
}

func TestConstants(t *testing.T) {
	if config.DefaultPort != 8080 {
		t.Errorf("DefaultPort = %d, want 8080", config.DefaultPort)
	}
	if config.DefaultTimeout != 15*time.Second {
		t.Errorf("DefaultTimeout = %v, want 15s", config.DefaultTimeout)
	}
	if config.DefaultRateLimit != 100 {
		t.Errorf("DefaultRateLimit = %d, want 100", config.DefaultRateLimit)
	}
	if config.PubKeyLen != 64 {
		t.Errorf("PubKeyLen = %d, want 64", config.PubKeyLen)
	}
	if config.FileChunkSize != 256<<10 {
		t.Errorf("FileChunkSize = %d, want 262144", config.FileChunkSize)
	}
	if config.FileIDLen != 16 {
		t.Errorf("FileIDLen = %d, want 16", config.FileIDLen)
	}
	if config.NonceSize != 12 {
		t.Errorf("NonceSize = %d, want 12", config.NonceSize)
	}
	if config.MultipartMemBuf != 10<<20 {
		t.Errorf("MultipartMemBuf = %d, want 10485760", config.MultipartMemBuf)
	}
	if config.DefaultClientName != "default" {
		t.Errorf("DefaultClientName = %q, want %q", config.DefaultClientName, "default")
	}
}
