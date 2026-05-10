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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		serverURL+"/api/v1/register-device",
		bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		var result struct {
			Error       string `json:"error"`
			DeviceCount int    `json:"device_count"`
			DeviceLimit int    `json:"device_limit"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("device limit reached (%d/%d). Please remove a device from your dashboard", result.DeviceCount, result.DeviceLimit)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("registration failed with status %d", resp.StatusCode)
	}

	return nil
}

// getDeviceName returns a human-readable device name
func getDeviceName() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "Unknown Device"
	}
	return hostname
}
