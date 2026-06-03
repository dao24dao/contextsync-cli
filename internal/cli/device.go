package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"contextsync/internal/config"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
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
			if result.DeviceLimit <= 1 {
				return fmt.Errorf("Free tier supports 1 device only. Upgrade to Pro for up to 3 devices with cloud sync")
			}
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

// deviceListCmd shows all registered devices for the current account.
var deviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered devices",
	Long:  "List all devices registered to your ContextSync account.",
	RunE: func(cmd *cobra.Command, args []string) error {
		runDeviceList()
		return nil
	},
}

// deviceRemoveCmd removes a device from your account.
var deviceRemoveCmd = &cobra.Command{
	Use:   "remove <device_id>",
	Short: "Remove a registered device",
	Long:  "Remove a device from your ContextSync account. Pro users can remove remote devices. Free tier users only have 1 device and cannot remove it.",
	Args:  cobra.ExactArgs(1),
	Run:   func(cmd *cobra.Command, args []string) { runDeviceRemove(args[0]) },
}

func runDeviceList() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	proStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))

	fmt.Println(titleStyle.Render("\nContextSync Devices\n"))

	if !config.IsLoggedIn() {
		fmt.Println(warnStyle.Render("  Not logged in. Please login first:"))
		fmt.Println("  Run: contextsync login")
		fmt.Println()
		return
	}

	ensureDatabase()

	token := config.GetAuthToken()
	serverURL := config.GetServerURL()
	currentDeviceID := config.GetDeviceID()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", serverURL+"/api/v1/devices", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Network error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Failed to fetch devices (HTTP %d)\n", resp.StatusCode)
		os.Exit(1)
	}

	var devices []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&devices)

	fmt.Printf("  %-4s %-35s %-20s %s\n", "ID", "DEVICE", "LAST SEEN", "STATUS")
	fmt.Println("  " + strings.Repeat("-", 75))

	for _, d := range devices {
		devID, _ := d["device_id"].(string)
		lastSeen, _ := d["last_seen"].(string)
		status := "remote"
		if devID == currentDeviceID {
			status = "this device"
		}
		isPro := validator.IsPro()
		maxDevices := 3
		if !isPro {
			maxDevices = 1
		}
		fmt.Printf("  %-4s %-35s %-20s %s (%d/%d)\n", devID[:min(len(devID), 35)], lastSeen, proStyle.Render(status), maxDevices)
	}

	fmt.Println()
	if validator.IsPro() {
		fmt.Printf("  Pro: up to 3 devices registered\n")
		fmt.Printf("  Manage: contextsync devices remove <device_id>\n")
	} else {
		fmt.Printf("  Free: 1 device only. Upgrade to Pro for 3 devices.\n")
	}
	fmt.Println()
}

func runDeviceRemove(deviceID string) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	fmt.Println(titleStyle.Render("\n  Removing Device\n"))

	if !config.IsLoggedIn() {
		fmt.Println(errorStyle.Render("  Not logged in. Please login first."))
		fmt.Println("  Run: contextsync login")
		fmt.Println()
		return
	}

	ensureDatabase()

	if deviceID == config.GetDeviceID() {
		fmt.Println(warnStyle.Render("  You cannot remove your current device."))
		fmt.Println()
		return
	}

	if !validator.IsPro() {
		fmt.Println(warnStyle.Render("  Free tier supports 1 device only. You cannot remove devices."))
		fmt.Println()
		fmt.Println("  Run: " + lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render("contextsync upgrade"))
		fmt.Println()
		return
	}

	token := config.GetAuthToken()
	serverURL := config.GetServerURL()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE", serverURL+"/api/v1/devices/"+deviceID, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Network error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Failed to remove device: %s", result.Error)))
		fmt.Println()
		return
	}

	fmt.Println(successStyle.Render("  Device removed successfully.\n"))
	fmt.Println("  Run 'contextsync sync' on other devices to update their device list.")
	fmt.Println()
}
