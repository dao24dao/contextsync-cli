package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update ContextSync CLI to the latest version",
	Long: `Update ContextSync CLI to the latest version from GitHub.

This command will:
  1. Check for the latest version on GitHub
  2. Download the new binary
  3. Replace the current installation`,
	Run: func(cmd *cobra.Command, args []string) {
		runUpdate()
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

	fmt.Println(titleStyle.Render("\nContextSync CLI Updater\n"))

	// Detect OS and Arch
	goos := runtime.GOOS
	arch := runtime.GOARCH

	// Normalize OS
	if goos != "darwin" && goos != "linux" && goos != "windows" {
		fmt.Println(errorStyle.Render("Unsupported OS: " + goos))
		os.Exit(1)
	}

	// Normalize Arch
	if arch == "x86_64" || arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" || arch == "aarch64" {
		arch = "arm64"
	} else {
		fmt.Println(errorStyle.Render("Unsupported architecture: " + arch))
		os.Exit(1)
	}

	fmt.Printf("Detected: %s/%s\n\n", goos, arch)

	// GitHub repo
	githubRepo := "dao24dao/contextsync-cli"
	baseURL := "https://github.com/" + githubRepo + "/releases"

	// Get latest version
	fmt.Println(infoStyle.Render("Checking for latest version..."))

	latestURL, err := getLatestReleaseURL(baseURL)
	if err != nil {
		fmt.Println(errorStyle.Render("Failed to get latest version: " + err.Error()))
		fmt.Println("\nYou can manually download from:")
		fmt.Printf("  %s/latest\n\n", baseURL)
		os.Exit(1)
	}

	// Extract version from URL
	latestVersion := extractVersion(latestURL)
	fmt.Printf("Latest version: %s\n", successStyle.Render(latestVersion))
	fmt.Printf("Current version: %s\n\n", version)

	// Build download URL
	var binaryName string
	if goos == "windows" {
		binaryName = fmt.Sprintf("contextsync-%s-%s.zip", goos, arch)
	} else {
		binaryName = fmt.Sprintf("contextsync-%s-%s.tar.gz", goos, arch)
	}

	downloadURL := fmt.Sprintf("%s/download/%s/%s", baseURL, latestVersion, binaryName)

	fmt.Println(infoStyle.Render("Downloading..."))
	fmt.Printf("  URL: %s\n\n", downloadURL)

	// Download to temp file
	tmpDir, err := os.MkdirTemp("", "contextsync-update-*")
	if err != nil {
		fmt.Println(errorStyle.Render("Failed to create temp directory: " + err.Error()))
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := tmpDir + "/" + binaryName

	if err := downloadFile(downloadURL, tmpFile); err != nil {
		fmt.Println(errorStyle.Render("Failed to download: " + err.Error()))
		os.Exit(1)
	}

	fmt.Println(successStyle.Render("Download complete!\n"))

	// Extract
	fmt.Println(infoStyle.Render("Extracting..."))

	extractedBinary, err := extractFile(tmpFile, tmpDir+"/contextsync", goos)
	if err != nil {
		fmt.Println(errorStyle.Render("Failed to extract: " + err.Error()))
		os.Exit(1)
	}

	fmt.Printf("Extracted binary: %s\n", extractedBinary)

	// Find current binary path
	currentBinary, err := os.Executable()
	if err != nil {
		fmt.Println(errorStyle.Render("Failed to find current binary: " + err.Error()))
		os.Exit(1)
	}

	fmt.Printf("Current binary: %s\n\n", currentBinary)

	// Replace binary
	fmt.Println(infoStyle.Render("Installing..."))

	if err := replaceBinary(extractedBinary, currentBinary); err != nil {
		fmt.Println(errorStyle.Render("Failed to install: " + err.Error()))
		fmt.Println("\nYou may need to run with sudo:")
		fmt.Printf("  sudo contextsync update\n\n")
		os.Exit(1)
	}

	fmt.Println(successStyle.Render("\n✅ ContextSync CLI updated successfully!"))
	fmt.Printf("\nNew version: %s\n", latestVersion)
	fmt.Println("\nRun 'contextsync version' to verify.\n")
}

func getLatestReleaseURL(baseURL string) (string, error) {
	// Follow redirects to get latest version URL
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(baseURL + "/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 302 {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no redirect location found")
	}

	return location, nil
}

func extractVersion(url string) string {
	parts := strings.Split(url, "/")
	for i, p := range parts {
		if p == "tag" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "unknown"
}

func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Progress bar
	counter := &writeCounter{}
	_, err = io.Copy(out, io.TeeReader(resp.Body, counter))
	fmt.Println() // New line after progress

	return err
}

type writeCounter struct {
	Total uint64
}

func (wc *writeCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	wc.PrintProgress()
	return n, nil
}

func (wc *writeCounter) PrintProgress() {
	// Print progress in MB
	fmt.Printf("\r    Downloaded: %.2f MB", float64(wc.Total)/1024/1024)
}

func extractFile(archive, expectedDest, goos string) (string, error) {
	// Get directory from expected destination
	dir := filepath.Dir(expectedDest)

	var cmd *exec.Cmd
	if goos == "windows" {
		cmd = exec.Command("powershell", "-c", fmt.Sprintf("Expand-Archive -Path %s -DestinationPath %s", archive, dir))
	} else {
		cmd = exec.Command("sh", "-c", fmt.Sprintf("cd %s && tar -xzf %s", dir, archive))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("extract failed: %s: %s", err, string(output))
	}

	// Find the extracted binary (may have platform suffix like contextsync-darwin-arm64)
	var extractedBinary string
	candidates := []string{
		expectedDest,                              // contextsync
		dir + "/contextsync",                      // contextsync
	}

	// Also check for platform-suffixed binary
	arch := runtime.GOARCH
	if arch == "x86_64" || arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" || arch == "aarch64" {
		arch = "arm64"
	}
	candidates = append(candidates, dir+"/contextsync-"+goos+"-"+arch)

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			extractedBinary = candidate
			break
		}
	}

	if extractedBinary == "" {
		// List files in directory for debugging
		files, _ := os.ReadDir(dir)
		fileList := make([]string, 0)
		for _, f := range files {
			fileList = append(fileList, f.Name())
		}
		return "", fmt.Errorf("could not find extracted binary in %s. Files: %v", dir, fileList)
	}

	// Make executable
	if err := os.Chmod(extractedBinary, 0755); err != nil {
		return "", fmt.Errorf("chmod failed: %w", err)
	}

	return extractedBinary, nil
}

func replaceBinary(newBinary, currentBinary string) error {
	// Try to rename first
	if err := os.Rename(newBinary, currentBinary); err == nil {
		return nil
	}

	// If rename fails (cross-device), try copy
	src, err := os.Open(newBinary)
	if err != nil {
		return err
	}
	defer src.Close()

	// Remove old binary first
	os.Remove(currentBinary)

	dst, err := os.OpenFile(currentBinary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}
