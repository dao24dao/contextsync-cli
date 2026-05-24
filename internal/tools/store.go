package tools

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AES key hardcoded in CLI binary (16 bytes for AES-128)
var encKey = []byte("CtxSync2026ToolK")

// ToolStore represents the encrypted tool configuration
type ToolStore struct {
	Tools     []string `json:"tools"`
	AccountID string   `json:"account_id"`
	UpdatedAt string   `json:"updated_at"`
}

// GetToolsPath returns the path to tools.enc
func GetToolsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".contextsync", "tools.enc")
}

// Save encrypts and writes the tool store to disk
func Save(store *ToolStore) error {
	store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	plaintext, err := json.Marshal(store)
	if err != nil {
		return fmt.Errorf("marshal tool store: %w", err)
	}

	encrypted, err := encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt tool store: %w", err)
	}

	path := GetToolsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	return os.WriteFile(path, []byte(encrypted), 0600)
}

// Load reads and decrypts the tool store from disk
func Load() (*ToolStore, error) {
	path := GetToolsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tools.enc: %w", err)
	}

	plaintext, err := decrypt(string(data))
	if err != nil {
		return nil, fmt.Errorf("decrypt tool store: %w", err)
	}

	var store ToolStore
	if err := json.Unmarshal(plaintext, &store); err != nil {
		return nil, fmt.Errorf("parse tool store: %w", err)
	}

	return &store, nil
}

// IsToolAllowed checks if a tool name is in the allowed list
func IsToolAllowed(toolName string) (bool, error) {
	store, err := Load()
	if err != nil {
		return false, err
	}

	for _, t := range store.Tools {
		if t == toolName {
			return true, nil
		}
	}
	return false, nil
}

// GetToolCount returns the number of configured tools
func GetToolCount() (int, error) {
	store, err := Load()
	if err != nil {
		return 0, err
	}
	return len(store.Tools), nil
}

// encrypt encrypts plaintext using AES-GCM and returns base64 encoded string
func encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a base64 encoded AES-GCM ciphertext
func decrypt(encoded string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
