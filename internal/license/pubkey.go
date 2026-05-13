package license

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	PublicKeyEndpoint = "/api/v1/public-key"
	PublicKeyCacheFile = "public_key.pem"
	PublicKeyMaxAge = 24 * time.Hour
)

var (
	cachedPublicKey ed25519.PublicKey
	pubKeyMutex     sync.RWMutex
	pubKeyFetchedAt time.Time
)

// PublicKeyResponse is the server response for public key
type PublicKeyResponse struct {
	PublicKey string `json:"public_key"`
	Algorithm string `json:"algorithm"`
}

// FetchPublicKey fetches the server's public key for signature verification
func FetchPublicKey(serverURL string) error {
	if serverURL == "" {
		return fmt.Errorf("server URL not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", serverURL+PublicKeyEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch public key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var result PublicKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Parse the PEM encoded public key
	block, _ := pem.Decode([]byte(result.PublicKey))
	if block == nil {
		// Try base64 decode
		pubKeyBytes, err := base64.StdEncoding.DecodeString(result.PublicKey)
		if err != nil {
			return fmt.Errorf("failed to decode public key: %w", err)
		}
		if len(pubKeyBytes) != ed25519.PublicKeySize {
			return fmt.Errorf("invalid public key size: %d", len(pubKeyBytes))
		}
		cachedPublicKey = ed25519.PublicKey(pubKeyBytes)
	} else {
		if len(block.Bytes) != ed25519.PublicKeySize {
			return fmt.Errorf("invalid public key size: %d", len(block.Bytes))
		}
		cachedPublicKey = ed25519.PublicKey(block.Bytes)
	}

	pubKeyFetchedAt = time.Now()

	// Save to file for offline use
	if err := savePublicKeyToFile(result.PublicKey); err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to cache public key: %v\n", err)
	}

	// Also set as embedded key
	embeddedPublicKey = cachedPublicKey

	return nil
}

// LoadCachedPublicKey loads the public key from cache file
func LoadCachedPublicKey() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cachePath := filepath.Join(homeDir, ".contextsync", PublicKeyCacheFile)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return err
	}

	// Try PEM decode first
	block, _ := pem.Decode(data)
	if block != nil {
		if len(block.Bytes) != ed25519.PublicKeySize {
			return fmt.Errorf("invalid cached public key size")
		}
		cachedPublicKey = ed25519.PublicKey(block.Bytes)
		embeddedPublicKey = cachedPublicKey
		return nil
	}

	// Try base64
	pubKeyBytes, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return fmt.Errorf("failed to decode cached public key: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid cached public key size")
	}

	cachedPublicKey = ed25519.PublicKey(pubKeyBytes)
	embeddedPublicKey = cachedPublicKey

	return nil
}

// savePublicKeyToFile saves the public key to cache file
func savePublicKeyToFile(pubKey string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cachePath := filepath.Join(homeDir, ".contextsync", PublicKeyCacheFile)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}

	return os.WriteFile(cachePath, []byte(pubKey), 0644)
}

// GetPublicKey returns the current public key
func GetPublicKey() ed25519.PublicKey {
	pubKeyMutex.RLock()
	defer pubKeyMutex.RUnlock()
	return cachedPublicKey
}

// EnsurePublicKey ensures a public key is available for verification
func EnsurePublicKey(serverURL string) error {
	pubKeyMutex.Lock()
	defer pubKeyMutex.Unlock()

	// Check if we have a valid cached key
	if cachedPublicKey != nil && time.Since(pubKeyFetchedAt) < PublicKeyMaxAge {
		return nil
	}

	// Try to load from file first
	if err := LoadCachedPublicKey(); err == nil {
		return nil
	}

	// Fetch from server
	return FetchPublicKey(serverURL)
}
