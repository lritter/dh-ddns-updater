//go:build integration
// +build integration

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRealIPService tests against the actual ipinfo.io service
// Run with: go test -tags=integration -v
func TestRealIPService(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	updater := &DDNSUpdater{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ip, err := updater.getCurrentIP(ctx)
	if err != nil {
		t.Fatalf("failed to get current IP: %v", err)
	}

	if ip == "" {
		t.Error("got empty IP address")
	}

	t.Logf("Current public IP: %s", ip)

	// Basic validation - should look like an IP
	if len(ip) < 7 || len(ip) > 15 {
		t.Errorf("IP address looks invalid: %q", ip)
	}
}

// TestConfigValidation tests loading the actual config file
func TestConfigValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test with example config
	configPath := "config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("config.yaml not found, skipping config validation test")
	}

	config, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Basic validation
	if config.DreamhostAPIKey == "" || config.DreamhostAPIKey == "YOUR_API_KEY_HERE" {
		t.Log("Warning: API key not configured in config.yaml")
	}

	if len(config.Domains) == 0 {
		t.Error("no domains configured")
	}

	for i, domain := range config.Domains {
		if domain.Name == "" {
			t.Errorf("domain %d has empty name", i)
		}
		if domain.Type == "" {
			t.Errorf("domain %d has empty type", i)
		}
		if domain.Type != "A" && domain.Type != "AAAA" {
			t.Logf("Warning: domain %d has unusual type %q", i, domain.Type)
		}
	}

	t.Logf("Config validation passed for %d domains", len(config.Domains))
}

// TestFullCycle tests a complete cycle without actually updating DNS
// This is useful for testing the flow with real external services
func TestFullCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if no API key is configured
	apiKey := os.Getenv("DREAMHOST_API_KEY")
	if apiKey == "" {
		t.Skip("DREAMHOST_API_KEY not set, skipping full cycle test")
	}

	tempDir, err := os.MkdirTemp("", "ddns-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test config
	config := &Config{
		CheckInterval:   1 * time.Minute,
		DreamhostAPIKey: apiKey,
		StatePath:       filepath.Join(tempDir, "state.json"),
		LogLevel:        "debug",
		Domains: []DomainConfig{
			// Use a test subdomain that won't break anything
			{Name: "example.com", Record: "ddns-test", Type: "A"},
		},
	}

	state := &State{
		Records: make(map[string]string),
	}

	updater := &DDNSUpdater{
		config:     config,
		state:      state,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test IP fetching
	ip, err := updater.getCurrentIP(ctx)
	if err != nil {
		t.Fatalf("failed to get current IP: %v", err)
	}

	t.Logf("Current IP: %s", ip)

	// Test state saving
	updater.state.LastIP = ip
	updater.state.LastUpdated = time.Now()

	if err := updater.saveState(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Test state loading
	loadedState, err := loadState(config.StatePath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loadedState.LastIP != ip {
		t.Errorf("expected loaded IP %q, got %q", ip, loadedState.LastIP)
	}

	t.Log("Full cycle test completed successfully (DNS update skipped)")
}
