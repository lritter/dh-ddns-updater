// Package main implements a dynamic DNS updater daemon for Dreamhost.
// It periodically checks the public IP address and updates DNS records when changes are detected.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// Default configuration and state file paths
const (
	DefaultConfigPath = "/etc/dh-ddns-updater/config.yaml"
	DefaultStatePath  = "/var/lib/dh-ddns-updater/state.json"
	IPInfoURL         = "https://ipinfo.io/ip"
	DreamhostAPIBase  = "https://api.dreamhost.com/"
)

// Config holds the daemon configuration loaded from YAML
type Config struct {
	CheckInterval   time.Duration  `yaml:"check_interval"`    // How often to check for IP changes
	Domains         []DomainConfig `yaml:"domains"`           // List of domains/records to update
	DreamhostAPIKey string         `yaml:"dreamhost_api_key"` // API key for Dreamhost
	StatePath       string         `yaml:"state_path"`        // Where to store persistent state
	LogLevel        string         `yaml:"log_level"`         // Logging level (debug, info, warn, error)
}

// DomainConfig represents a single DNS record to manage
type DomainConfig struct {
	Name   string `yaml:"name"`   // Domain name (e.g., "example.com")
	Type   string `yaml:"type"`   // Record type (e.g., "A", "AAAA")
	Record string `yaml:"record"` // Subdomain/record name (e.g., "home" for home.example.com, "" for apex)
}

// State holds persistent data between daemon runs
type State struct {
	LastIP      string            `json:"last_ip"`      // Last known public IP address
	LastUpdated time.Time         `json:"last_updated"` // When records were last updated
	Records     map[string]string `json:"records"`      // Map of record names to their current IP values
}

// IPInfoResponse represents the JSON response from ipinfo.io
type IPInfoResponse struct {
	IP string `json:"ip"`
}

// DreamhostResponse represents the JSON response from Dreamhost API
type DreamhostResponse struct {
	Result string `json:"result"` // "success" or "error"
	Data   string `json:"data"`   // Response message or error details
}

// DDNSUpdater is the main daemon struct that orchestrates IP checking and DNS updates
type DDNSUpdater struct {
	config     *Config
	state      *State
	httpClient *http.Client
	logger     *slog.Logger
}

// NewDDNSUpdater creates and initializes a new DDNSUpdater instance.
// It loads configuration from the specified path, sets up logging, and loads
// any existing state from disk. Returns an error if configuration is invalid.
func NewDDNSUpdater(configPath string) (*DDNSUpdater, error) {
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// Set defaults
	if config.CheckInterval == 0 {
		config.CheckInterval = 5 * time.Minute
	}
	if config.StatePath == "" {
		config.StatePath = DefaultStatePath
	}
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	// Setup logging
	var level slog.Level
	switch strings.ToLower(config.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	state, err := loadState(config.StatePath)
	if err != nil {
		return nil, fmt.Errorf("loading state: %w", err)
	}

	return &DDNSUpdater{
		config: config,
		state:  state,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}, nil
}

// Run starts the main daemon loop. It performs an initial IP check, then runs
// on a timer checking for IP changes at the configured interval. The loop continues
// until the context is cancelled (typically by a signal handler).
func (d *DDNSUpdater) Run(ctx context.Context) error {
	d.logger.Info("Starting DDNS updater",
		"check_interval", d.config.CheckInterval,
		"domains", len(d.config.Domains))

	ticker := time.NewTicker(d.config.CheckInterval)
	defer ticker.Stop()

	// Do initial check
	if err := d.checkAndUpdate(ctx); err != nil {
		d.logger.Error("Initial check failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("Shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := d.checkAndUpdate(ctx); err != nil {
				d.logger.Error("Check and update failed", "error", err)
			}
		}
	}
}

// checkAndUpdate performs one cycle of IP checking and DNS updating.
// It fetches the current public IP, compares it to the last known IP,
// and updates all configured DNS records if the IP has changed.
// Returns an error if any critical operations fail.
func (d *DDNSUpdater) checkAndUpdate(ctx context.Context) error {
	currentIP, err := d.getCurrentIP(ctx)
	if err != nil {
		return fmt.Errorf("getting current IP: %w", err)
	}

	d.logger.Debug("Current IP", "ip", currentIP)

	// Log IP change if it occurred, but don't exit early
	if currentIP != d.state.LastIP {
		d.logger.Info("IP changed", "old", d.state.LastIP, "new", currentIP)
	}

	var updateErrors []error
	updatedAnyRecord := false

	for _, domain := range d.config.Domains {
		recordKey := fmt.Sprintf("%s.%s", domain.Record, domain.Name)
		if domain.Record == "" {
			recordKey = domain.Name
		}

		// Always check current DNS record value
		currentRecordIP, err := d.getCurrentDNSRecord(ctx, domain)
		if err != nil {
			d.logger.Warn("Failed to get current DNS record, will update anyway",
				"domain", domain.Name,
				"record", domain.Record,
				"error", err)
			currentRecordIP = "" // Force update if we can't check
		}

		// If the record already has the correct IP, just move on.
		if currentRecordIP == currentIP {
			d.logger.Debug("DNS record already up to date",
				"domain", domain.Name,
				"record", domain.Record,
				"ip", currentIP)
			d.state.Records[recordKey] = currentIP
			continue
		}

		d.logger.Info("Updating DNS record",
			"domain", domain.Name,
			"record", domain.Record,
			"old_ip", currentRecordIP,
			"new_ip", currentIP)

		if err := d.updateDNSRecord(ctx, domain, currentIP); err != nil {
			d.logger.Error("Failed to update DNS record",
				"domain", domain.Name,
				"record", domain.Record,
				"error", err)
			updateErrors = append(updateErrors, err)
		} else {
			d.logger.Info("Successfully updated DNS record",
				"domain", domain.Name,
				"record", domain.Record,
				"ip", currentIP)
			d.state.Records[recordKey] = currentIP
			updatedAnyRecord = true
		}
	}

	// Update state if we successfully processed everything
	if len(updateErrors) == 0 {
		d.state.LastIP = currentIP
		if updatedAnyRecord {
			d.state.LastUpdated = time.Now()
		}

		if err := d.saveState(); err != nil {
			d.logger.Error("Failed to save state", "error", err)
		}
	}

	if len(updateErrors) > 0 {
		return fmt.Errorf("failed to update %d records", len(updateErrors))
	}

	return nil
}

// getCurrentDNSRecord fetches the current value of a DNS record from Dreamhost.
// Returns the current IP address for the record, or an empty string if the record
// doesn't exist or if there's an error fetching it.
func (d *DDNSUpdater) getCurrentDNSRecord(ctx context.Context, domain DomainConfig) (string, error) {
	params := url.Values{}
	params.Set("key", d.config.DreamhostAPIKey)
	params.Set("cmd", "dns-list_records")
	params.Set("format", "json")

	apiURL := DreamhostAPIBase + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from Dreamhost API", resp.StatusCode)
	}

	var dhResp struct {
		Result string `json:"result"`
		Data   []struct {
			Record string `json:"record"`
			Type   string `json:"type"`
			Value  string `json:"value"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&dhResp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if dhResp.Result != "success" {
		return "", fmt.Errorf("dreamhost API error")
	}

	// Find the matching record
	targetRecord := domain.Name
	if domain.Record != "" {
		targetRecord = fmt.Sprintf("%s.%s", domain.Record, domain.Name)
	}

	for _, record := range dhResp.Data {
		if record.Record == targetRecord && record.Type == domain.Type {
			return record.Value, nil
		}
	}

	// Record not found
	return "", nil
}

// Returns the IP as a string, or an error if the request fails or
// returns an unexpected response.
func (d *DDNSUpdater) getCurrentIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", IPInfoURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from ipinfo.io", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return "", fmt.Errorf("empty response from ipinfo.io")
	}

	return ip, nil
}

// updateDNSRecord updates a single DNS record via the Dreamhost API.
// It first attempts to remove any existing record with the same name and type,
// then adds a new record with the current IP address. This approach handles
// cases where the record already exists with a different IP.
func (d *DDNSUpdater) updateDNSRecord(ctx context.Context, domain DomainConfig, ip string) error {
	// First, remove existing record if it exists
	if err := d.removeDNSRecord(ctx, domain); err != nil {
		d.logger.Warn("Failed to remove existing record (might not exist)",
			"domain", domain.Name, "record", domain.Record, "error", err)
	}

	// Add new record
	params := url.Values{}
	params.Set("key", d.config.DreamhostAPIKey)
	params.Set("cmd", "dns-add_record")
	params.Set("record", domain.Record)
	params.Set("type", domain.Type)
	params.Set("value", ip)
	params.Set("format", "json")

	if domain.Record != "" {
		params.Set("record", fmt.Sprintf("%s.%s", domain.Record, domain.Name))
	} else {
		params.Set("record", domain.Name)
	}

	apiURL := DreamhostAPIBase + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from Dreamhost API", resp.StatusCode)
	}

	var dhResp DreamhostResponse
	if err := json.NewDecoder(resp.Body).Decode(&dhResp); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if dhResp.Result != "success" {
		return fmt.Errorf("dreamhost API error: %s", dhResp.Data)
	}

	return nil
}

// removeDNSRecord attempts to remove an existing DNS record via the Dreamhost API.
// This is called before adding a new record to ensure we don't have duplicates.
// Failures are not considered fatal since the record might not exist.
func (d *DDNSUpdater) removeDNSRecord(ctx context.Context, domain DomainConfig) error {
	params := url.Values{}
	params.Set("key", d.config.DreamhostAPIKey)
	params.Set("cmd", "dns-remove_record")
	params.Set("record", domain.Record)
	params.Set("type", domain.Type)
	params.Set("format", "json")

	if domain.Record != "" {
		params.Set("record", fmt.Sprintf("%s.%s", domain.Record, domain.Name))
	} else {
		params.Set("record", domain.Name)
	}

	// Get current value first
	currentIP, exists := d.state.Records[fmt.Sprintf("%s.%s", domain.Record, domain.Name)]
	if exists {
		params.Set("value", currentIP)
	}

	apiURL := DreamhostAPIBase + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Don't treat this as fatal - record might not exist
	return nil
}

// saveState persists the current state to disk as JSON.
// Creates the state directory if it doesn't exist. The state includes
// the last known IP and timestamp to avoid unnecessary API calls.
func (d *DDNSUpdater) saveState() error {
	if err := os.MkdirAll(strings.TrimSuffix(d.config.StatePath, "/state.json"), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(d.state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(d.config.StatePath, data, 0644)
}

// loadConfig reads and parses the YAML configuration file.
// Returns a Config struct or an error if the file cannot be read or parsed.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// loadState reads and parses the JSON state file.
// If the state file doesn't exist, creates a new one with default values.
// Creates the directory structure if it doesn't exist.
// Returns a State struct or an error if file operations fail.
func loadState(path string) (*State, error) {
	// Try to read existing state file
	data, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist, create it with default state
		if os.IsNotExist(err) {
			// Create directory structure if it doesn't exist
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("creating state directory: %w", err)
			}

			// Create default state
			defaultState := &State{
				Records: make(map[string]string),
			}

			// Write default state to file
			data, err := json.MarshalIndent(defaultState, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshaling default state: %w", err)
			}

			if err := os.WriteFile(path, data, 0644); err != nil {
				return nil, fmt.Errorf("creating state file: %w", err)
			}

			return defaultState, nil
		}
		// Return other read errors as-is
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	// Parse existing state file
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	// Ensure Records map is initialized
	if state.Records == nil {
		state.Records = make(map[string]string)
	}

	return &state, nil
}

// main is the entry point for the daemon. It initializes the updater,
// sets up signal handling for graceful shutdown, and starts the main run loop.
// Takes an optional config file path as the first command line argument.
func main() {
	configPath := DefaultConfigPath
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	updater, err := NewDDNSUpdater(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize updater: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		updater.logger.Info("Received signal", "signal", sig)
		cancel()
	}()

	if err := updater.Run(ctx); err != nil && err != context.Canceled {
		updater.logger.Error("Updater failed", "error", err)
		os.Exit(1)
	}
}
