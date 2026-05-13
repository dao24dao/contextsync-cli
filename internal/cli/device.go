package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"contextsync/internal/config"
)

// registerDevice registers the current device with the server
func registerDevice() error {
	if !config.IsLoggedIn() {
		return fmt.Errorf("not logged in")
	}

	serverURL := config.GetServerURL()
	accountID := config.GetAccountID()
	deviceID := config.GetDeviceID()
	token := config.GetAuthToken()

	if accountID == "" || deviceID == "" {
		return fmt.Errorf("missing account or device ID")
	}

	body := map[string]string{
		"account_id":  accountID,
		"device_id":   deviceID,
		"device_name": getDeviceName(),
	}

	jsonBody, _ := json.Marshal(body)

	var lastErr error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		req, err := http.NewRequestWithContext(ctx, "POST",
			serverURL+"/api/v1/register-device",
			bytes.NewReader(jsonBody))
		if err != nil {
			cancel()
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("failed to connect to server (attempt %d/%d): %w", attempt, maxRetries, err)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * time.Second)
			}
			continue
		}
		cancel()

		if resp.StatusCode == 403 {
			var result struct {
				Error       string `json:"error"`
				DeviceCount int    `json:"device_count"`
				DeviceLimit int    `json:"device_limit"`
			}
			json.NewDecoder(resp.Body).Decode(&result)
			resp.Body.Close()
			return fmt.Errorf("device limit reached (%d/%d). Please remove a device from your dashboard", result.DeviceCount, result.DeviceLimit)
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			lastErr = fmt.Errorf("registration failed with status %d (attempt %d/%d)", resp.StatusCode, attempt, maxRetries)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * time.Second)
			}
			continue
		}

		resp.Body.Close()
		return nil
	}

	return lastErr
}

// getDeviceName returns a human-readable device name
func getDeviceName() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "Unknown Device"
	}
	return hostname
}
