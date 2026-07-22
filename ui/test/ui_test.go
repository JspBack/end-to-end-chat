package main_test

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLExists(t *testing.T) {
	raw, err := os.ReadFile("../index.html")
	if err != nil {
		t.Fatal("reading index.html:", err)
	}
	if len(raw) == 0 {
		t.Fatal("index.html is empty")
	}
}

func TestHTMLContainsTarget(t *testing.T) {
	raw, _ := os.ReadFile("../index.html")
	if !strings.Contains(string(raw), "{{TARGET}}") {
		t.Error("index.html should contain {{TARGET}} placeholder")
	}
}

func TestHTMLHasRequiredSections(t *testing.T) {
	raw, _ := os.ReadFile("../index.html")
	sections := []string{"Peers", "Sessions", "Connect", "Send", "Outbox", "Messages", "Search"}
	html := string(raw)
	for _, s := range sections {
		if !strings.Contains(html, s) {
			t.Errorf("index.html missing section: %s", s)
		}
	}
}

func TestTargetReplacement(t *testing.T) {
	raw, _ := os.ReadFile("../index.html")
	page := strings.ReplaceAll(string(raw), "{{TARGET}}", "1.2.3.4:5678")
	if !strings.Contains(page, "1.2.3.4:5678") {
		t.Error("replaced target not found in page")
	}
	if strings.Contains(page, "{{TARGET}}") {
		t.Error("{{TARGET}} placeholder should have been replaced")
	}
}
