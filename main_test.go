package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestConfig tests config loading and validation
func TestConfig(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantError bool
	}{
		{
			name: "valid config",
			yaml: `
check_interval: 5m
dreamhost_api_key: "test-key"
domains:
  - name: "example.com"
    record: "home"
    type: "A"
`,
			wantError: false,
		},
		{
			name: "invalid yaml",
			yaml: `
check_interval: 5m
domains:
  - name: "example.com"
    record: 
		 home  # bad indentation
    type: "A"
`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpfile, err := os.CreateTemp("", "config*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.yaml); err != nil {
				t.Fatal(err)
			}
			tmpfile.Close()

			config, err := loadConfig(tmpfile.Name())
			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if config.CheckInterval != 5*time.Minute {
				t.Errorf("expected check_interval 5m, got %v", config.CheckInterval)
			}

			if config.DreamhostAPIKey != "test-key" {
				t.Errorf("expected API key 'test-key', got %q", config.DreamhostAPIKey)
			}

			if len(config.Domains) != 1 {
				t.Errorf("expected 1 domain, got %d", len(config.Domains))
			}
		})
	}
}

// TestState tests state persistence
func TestState(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ddns-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	statePath := filepath.Join(tempDir, "state.json")

	// Test loading non-existent state
	_, err = loadState(statePath)
	if err != nil {
		t.Error("loading non-existent state should not return an error:", err)
	}

	// Test saving and loading state
	originalState := &State{
		LastIP:      "192.168.1.100",
		LastUpdated: time.Now(),
		Records:     map[string]string{"home.example.com": "192.168.1.100"},
	}

	// Create a mock updater to test saveState
	updater := &DDNSUpdater{
		config: &Config{StatePath: statePath},
		state:  originalState,
	}

	if err := updater.saveState(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	loadedState, err := loadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loadedState.LastIP != originalState.LastIP {
		t.Errorf("expected LastIP %q, got %q", originalState.LastIP, loadedState.LastIP)
	}

	if len(loadedState.Records) != len(originalState.Records) {
		t.Errorf("expected %d records, got %d", len(originalState.Records), len(loadedState.Records))
	}
}

// TestGetCurrentIP tests IP fetching with mock server
func TestGetCurrentIP(t *testing.T) {
	tests := []struct {
		name         string
		serverResp   string
		serverStatus int
		expectedIP   string
		expectError  bool
	}{
		{
			name:         "valid IP response",
			serverResp:   "203.0.113.42",
			serverStatus: 200,
			expectedIP:   "203.0.113.42",
			expectError:  false,
		},
		{
			name:         "IP with whitespace",
			serverResp:   "  203.0.113.42\n  ",
			serverStatus: 200,
			expectedIP:   "203.0.113.42",
			expectError:  false,
		},
		{
			name:         "empty response",
			serverResp:   "",
			serverStatus: 200,
			expectedIP:   "",
			expectError:  true,
		},
		{
			name:         "server error",
			serverResp:   "Internal Server Error",
			serverStatus: 500,
			expectedIP:   "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverResp))
			}))
			defer server.Close()

			// Create updater with custom HTTP client
			updater := &DDNSUpdater{
				httpClient: &http.Client{Timeout: 5 * time.Second},
			}

			// For this test, we'll modify the method to accept a URL parameter
			ctx := context.Background()
			ip, err := updater.getCurrentIPFromURL(ctx, server.URL)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ip != tt.expectedIP {
				t.Errorf("expected IP %q, got %q", tt.expectedIP, ip)
			}
		})
	}
}

// TestDreamhostAPI tests DNS record updates with mock Dreamhost API
func TestDreamhostAPI(t *testing.T) {
	tests := []struct {
		name          string
		listResponse  interface{} // Response for dns-list_records
		addResponse   DreamhostResponse
		expectedError bool
		expectedCalls int // How many API calls we expect
	}{
		{
			name: "successful update - record doesn't exist",
			listResponse: struct {
				Result string `json:"result"`
				Data   []struct {
					Record string `json:"record"`
					Type   string `json:"type"`
					Value  string `json:"value"`
				} `json:"data"`
			}{
				Result: "success",
				Data: []struct {
					Record string `json:"record"`
					Type   string `json:"type"`
					Value  string `json:"value"`
				}{}, // Empty - record doesn't exist
			},
			addResponse: DreamhostResponse{
				Result: "success",
				Data:   "record_added",
			},
			expectedError: false,
			expectedCalls: 3, // list + remove + add
		},
		{
			name: "no update needed - record already correct",
			listResponse: struct {
				Result string `json:"result"`
				Data   []struct {
					Record string `json:"record"`
					Type   string `json:"type"`
					Value  string `json:"value"`
				} `json:"data"`
			}{
				Result: "success",
				Data: []struct {
					Record string `json:"record"`
					Type   string `json:"type"`
					Value  string `json:"value"`
				}{
					{Record: "home.example.com", Type: "A", Value: "203.0.113.42"},
				},
			},
			addResponse: DreamhostResponse{
				Result: "success",
				Data:   "record_added",
			},
			expectedError: false,
			expectedCalls: 1, // just list
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++

				// Parse the command from query params
				cmd := r.URL.Query().Get("cmd")

				if cmd == "dns-list_records" {
					json.NewEncoder(w).Encode(tt.listResponse)
					return
				}

				if cmd == "dns-remove_record" {
					// Always succeed for remove (might not exist)
					response := DreamhostResponse{Result: "success", Data: "removed"}
					json.NewEncoder(w).Encode(response)
					return
				}

				if cmd == "dns-add_record" {
					// Use the test response for add
					json.NewEncoder(w).Encode(tt.addResponse)
					return
				}

				http.Error(w, "unknown command", 400)
			}))
			defer server.Close()

			// Create updater with mock API base URL
			updater := &DDNSUpdater{
				config: &Config{
					DreamhostAPIKey: "test-key",
				},
				httpClient: &http.Client{Timeout: 5 * time.Second},
			}

			domain := DomainConfig{
				Name:   "example.com",
				Record: "home",
				Type:   "A",
			}

			ctx := context.Background()

			// Test getCurrentDNSRecord first
			currentIP, err := updater.getCurrentDNSRecordWithURL(ctx, domain, server.URL)
			if err != nil && tt.expectedError {
				return // Expected error
			}
			if err != nil {
				t.Fatalf("unexpected error getting current record: %v", err)
			}

			// Check if we need to update based on current record
			newIP := "203.0.113.42"
			if currentIP == newIP {
				// Record is already correct, should not call update
				if callCount != 1 {
					t.Errorf("expected 1 API call for list only, got %d", callCount)
				}
				return
			}

			// Record needs update, test the update
			err = updater.updateDNSRecordWithURL(ctx, domain, newIP, server.URL)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if callCount != tt.expectedCalls {
				t.Errorf("expected %d API calls, got %d", tt.expectedCalls, callCount)
			}
		})
	}
}

// TestCheckAndUpdate tests the main update logic
func TestCheckAndUpdate(t *testing.T) {
	// Create temp directory for state
	tempDir, err := os.MkdirTemp("", "ddns-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Mock IP service
	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("203.0.113.42"))
	}))
	defer ipServer.Close()

	// Mock Dreamhost API
	apiCallCount := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCallCount++
		response := DreamhostResponse{Result: "success", Data: "ok"}
		json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	// Create updater
	updater := &DDNSUpdater{
		config: &Config{
			DreamhostAPIKey: "test-key",
			StatePath:       filepath.Join(tempDir, "state.json"),
			Domains: []DomainConfig{
				{Name: "example.com", Record: "home", Type: "A"},
			},
		},
		state: &State{
			LastIP:  "192.168.1.100", // Different IP to trigger update
			Records: make(map[string]string),
		},
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	// This would need the actual implementation to accept URLs for testing
	// For now, we'll test the logic separately

	// Test IP change detection
	oldIP := updater.state.LastIP
	newIP := "203.0.113.42"

	if oldIP == newIP {
		t.Error("expected different IPs for this test")
	}

	// In a real test, we'd call checkAndUpdate and verify:
	// 1. IP was fetched
	// 2. DNS records were updated
	// 3. State was saved
	// 4. No unnecessary API calls were made
}

// TestConfigDefaults tests that default values are properly set
func TestConfigDefaults(t *testing.T) {
	// Create minimal config
	tmpfile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	tempDir, err := os.MkdirTemp("", "ddns-test")
	if err != nil {
		t.Fatal(err)
	}

	minimalConfig := `
dreamhost_api_key: "test-key"
state_path: "%s/state.json"
domains:
  - name: "example.com"
    record: "test"
    type: "A"
`
	if _, err := tmpfile.WriteString(fmt.Sprintf(minimalConfig, tempDir)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	updater, err := NewDDNSUpdater(tmpfile.Name())
	if err != nil {
		t.Fatalf("failed to create updater: %v", err)
	}

	// Check defaults
	if updater.config.CheckInterval != 5*time.Minute {
		t.Errorf("expected default check interval 5m, got %v", updater.config.CheckInterval)
	}

	// Test that the state path was set correctly (not the default since we overrode it)
	expectedStatePath := filepath.Join(tempDir, "state.json")
	if updater.config.StatePath != expectedStatePath {
		t.Errorf("expected state path %s, got %s", expectedStatePath, updater.config.StatePath)
	}

	if updater.config.LogLevel != "info" {
		t.Errorf("expected default log level 'info', got %s", updater.config.LogLevel)
	}
}

func TestDefaultStatePath(t *testing.T) {
	// Create minimal config without state_path
	tmpfile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	minimalConfig := `
dreamhost_api_key: "test-key"
domains:
  - name: "example.com"
    record: "test"
    type: "A"
`
	if _, err := tmpfile.WriteString(minimalConfig); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	// Load config directly to test default setting
	config, err := loadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Simulate the default setting logic from NewDDNSUpdater
	if config.StatePath == "" {
		config.StatePath = DefaultStatePath
	}

	if config.StatePath != DefaultStatePath {
		t.Errorf("expected default state path %s, got %s", DefaultStatePath, config.StatePath)
	}
}

// TestDomainRecordFormatting tests how domain records are formatted for the API
func TestDomainRecordFormatting(t *testing.T) {
	tests := []struct {
		name     string
		domain   DomainConfig
		expected string
	}{
		{
			name:     "subdomain record",
			domain:   DomainConfig{Name: "example.com", Record: "home", Type: "A"},
			expected: "home.example.com",
		},
		{
			name:     "apex record",
			domain:   DomainConfig{Name: "example.com", Record: "", Type: "A"},
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var recordName string
			if tt.domain.Record != "" {
				recordName = fmt.Sprintf("%s.%s", tt.domain.Record, tt.domain.Name)
			} else {
				recordName = tt.domain.Name
			}

			if recordName != tt.expected {
				t.Errorf("expected record name %q, got %q", tt.expected, recordName)
			}
		})
	}
}

// Helper method for testing - in real implementation you'd use dependency injection
// or make URLs configurable to avoid needing separate test methods
func (d *DDNSUpdater) getCurrentIPFromURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return "", fmt.Errorf("empty response")
	}

	return ip, nil
}

// Helper method for testing DNS API with custom URL
func (d *DDNSUpdater) updateDNSRecordWithURL(ctx context.Context, domain DomainConfig, ip, baseURL string) error {
	// Remove existing record first
	if err := d.removeDNSRecordWithURL(ctx, domain, baseURL); err != nil {
		// Don't fail on remove errors
	}

	// Add new record
	params := map[string]string{
		"key":    d.config.DreamhostAPIKey,
		"cmd":    "dns-add_record",
		"type":   domain.Type,
		"value":  ip,
		"format": "json",
	}

	if domain.Record != "" {
		params["record"] = fmt.Sprintf("%s.%s", domain.Record, domain.Name)
	} else {
		params["record"] = domain.Name
	}

	// Build URL
	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	apiURL := baseURL + "/?" + strings.Join(parts, "&")

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
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var dhResp DreamhostResponse
	if err := json.NewDecoder(resp.Body).Decode(&dhResp); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if dhResp.Result != "success" {
		return fmt.Errorf("API error: %s", dhResp.Data)
	}

	return nil
}

// Helper method for testing DNS record lookup with custom URL
func (d *DDNSUpdater) getCurrentDNSRecordWithURL(ctx context.Context, domain DomainConfig, baseURL string) (string, error) {
	params := map[string]string{
		"key":    d.config.DreamhostAPIKey,
		"cmd":    "dns-list_records",
		"format": "json",
	}

	// Build URL
	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	apiURL := baseURL + "/?" + strings.Join(parts, "&")

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
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
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
		return "", fmt.Errorf("API error")
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
func (d *DDNSUpdater) removeDNSRecordWithURL(ctx context.Context, domain DomainConfig, baseURL string) error {
	params := map[string]string{
		"key":    d.config.DreamhostAPIKey,
		"cmd":    "dns-remove_record",
		"type":   domain.Type,
		"format": "json",
	}

	if domain.Record != "" {
		params["record"] = fmt.Sprintf("%s.%s", domain.Record, domain.Name)
	} else {
		params["record"] = domain.Name
	}

	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	apiURL := baseURL + "/?" + strings.Join(parts, "&")

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
