package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"contextsync/internal/memory"
)

type Client struct {
	serverURL  string
	httpClient *http.Client
}

func NewClient(serverURL string) *Client {
	return &Client{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type SyncRequest struct {
	LicenseKey string       `json:"license_key"`
	DeviceID   string       `json:"device_id"`
	Memories   []SyncMemory `json:"memories"`
	LastSync   int64        `json:"last_sync"`
}

type SyncMemory struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	Source    string    `json:"source"`
	Project   string    `json:"project,omitempty"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeviceID  string    `json:"device_id"`
}

type SyncResponse struct {
	Success    bool         `json:"success"`
	Memories   []SyncMemory `json:"memories"`
	DeletedIDs []string     `json:"deleted_ids"`
	SyncedAt   int64        `json:"synced_at"`
	Error      string       `json:"error,omitempty"`
}

// Upload uploads local memories to the cloud
func (c *Client) Upload(ctx context.Context, licenseKey, deviceID string, memories []*memory.Memory) (*SyncResponse, error) {
	// Convert memories to sync format
	syncMemories := make([]SyncMemory, len(memories))
	for i, m := range memories {
		syncMemories[i] = SyncMemory{
			ID:        m.ID,
			Content:   m.Content,
			Category:  string(m.Category),
			Source:    m.Source,
			Project:   m.Project,
			Tags:      m.Tags,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
			DeviceID:  m.DeviceID,
		}
	}

	req := SyncRequest{
		LicenseKey: licenseKey,
		DeviceID:   deviceID,
		Memories:   syncMemories,
		LastSync:   time.Now().Unix(),
	}

	jsonBody, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.serverURL+"/api/v1/sync/upload",
		bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return &SyncResponse{Error: result.Error}, fmt.Errorf("sync failed: %s", result.Error)
	}

	return &result, nil
}

// Download downloads memories from the cloud
func (c *Client) Download(ctx context.Context, licenseKey, deviceID string, lastSync int64) (*SyncResponse, error) {
	req := SyncRequest{
		LicenseKey: licenseKey,
		DeviceID:   deviceID,
		LastSync:   lastSync,
	}

	jsonBody, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.serverURL+"/api/v1/sync/download",
		bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return &SyncResponse{Error: result.Error}, fmt.Errorf("download failed: %s", result.Error)
	}

	return &result, nil
}

// MergeAndSync performs a bidirectional sync
func (c *Client) MergeAndSync(ctx context.Context, licenseKey, deviceID string, localMemories []*memory.Memory, lastSync int64) ([]*memory.Memory, []string, error) {
	// First, download remote changes
	downloadResult, err := c.Download(ctx, licenseKey, deviceID, lastSync)
	if err != nil {
		return nil, nil, fmt.Errorf("download failed: %w", err)
	}

	// Then, upload local changes
	_, err = c.Upload(ctx, licenseKey, deviceID, localMemories)
	if err != nil {
		return nil, nil, fmt.Errorf("upload failed: %w", err)
	}

	// Convert downloaded memories to memory.Memory
	remoteMemories := make([]*memory.Memory, len(downloadResult.Memories))
	for i, m := range downloadResult.Memories {
		remoteMemories[i] = &memory.Memory{
			ID:        m.ID,
			Content:   m.Content,
			Category:  memory.Category(m.Category),
			Source:    m.Source,
			Project:   m.Project,
			Tags:      m.Tags,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
			DeviceID:  m.DeviceID,
			Synced:    true,
		}
	}

	return remoteMemories, downloadResult.DeletedIDs, nil
}
