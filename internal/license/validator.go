package license

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"contextsync/internal/db"
)

const (
	TrialDays = 14
)

type Validator struct {
	db         *sql.DB
	serverURL  string
	httpClient *http.Client
	cache      *LicenseCache
	mu         sync.RWMutex
}

type LicenseCache struct {
	Tier           string    `json:"tier"`
	SubscriptionID string    `json:"subscription_id"` // monthly, quarterly, yearly
	Valid          bool      `json:"valid"`
	ExpiresAt      time.Time `json:"expires_at"`
	FirstSeenAt    time.Time `json:"first_seen_at"`
	CachedAt       time.Time `json:"cached_at"`
}

type LicenseStatus struct {
	Valid          bool      `json:"valid"`
	Tier           string    `json:"tier"`
	SubscriptionID string    `json:"subscription_id"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type SubscriptionType string

const (
	SubscriptionMonthly   SubscriptionType = "monthly"
	SubscriptionQuarterly SubscriptionType = "quarterly"
	SubscriptionYearly    SubscriptionType = "yearly"
)

type SubscriptionPlan struct {
	ID           SubscriptionType
	Name         string
	Price        int    // in cents
	PriceDisplay string
	Savings      string
}

var SubscriptionPlans = []SubscriptionPlan{
	{SubscriptionMonthly, "Monthly", 900, "$9/month", ""},
	{SubscriptionQuarterly, "Quarterly", 2400, "$24/quarter", "Save 11%"},
	{SubscriptionYearly, "Yearly", 7200, "$72/year", "Save 33%"},
}

type Features struct {
	MaxTools        int    `json:"max_tools"`
	MemoryRetention string `json:"memory_retention"`
	CanSync         bool   `json:"can_sync"`
	CanSaveMemory   bool   `json:"can_save_memory"`
	TrialExpired    bool   `json:"trial_expired"`
}

// NewValidator creates a new license validator
func NewValidator(serverURL string) *Validator {
	return &Validator{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetDB sets the database for the validator
func (v *Validator) SetDB(database *db.SQLite) {
	v.db = database.DB()
	v.loadFromDB()
}

func (v *Validator) loadFromDB() {
	var tier, licenseKey string
	var validUntil, firstSeenAt, subscriptionType sql.NullString

	err := v.db.QueryRow(`
		SELECT tier, license_key, valid_until, first_seen_at, subscription_type FROM license WHERE id = 1
	`).Scan(&tier, &licenseKey, &validUntil, &firstSeenAt, &subscriptionType)

	if err == nil {
		v.cache = &LicenseCache{
			Tier: tier,
		}
		if validUntil.Valid {
			v.cache.ExpiresAt, _ = time.Parse(time.RFC3339, validUntil.String)
			v.cache.Valid = v.cache.ExpiresAt.After(time.Now())
		}
		if firstSeenAt.Valid {
			v.cache.FirstSeenAt, _ = time.Parse(time.RFC3339, firstSeenAt.String)
		}
		if subscriptionType.Valid {
			v.cache.SubscriptionID = subscriptionType.String
		}
	}
}

// IsPro checks if the user has a valid Pro license
func (v *Validator) IsPro() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Check cache first (valid for 24 hours)
	if v.cache != nil && time.Since(v.cache.CachedAt) < 24*time.Hour {
		return v.cache.Tier == "pro" && v.cache.Valid
	}

	// Validate with server
	if v.serverURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		status, err := v.validateWithServer(ctx)
		if err == nil {
			v.cache = &LicenseCache{
				Tier:           status.Tier,
				SubscriptionID: status.SubscriptionID,
				Valid:          status.Valid,
				ExpiresAt:      status.ExpiresAt,
				CachedAt:       time.Now(),
			}
			return status.Tier == "pro" && status.Valid
		}
	}

	// Fall back to cached value
	if v.cache != nil {
		return v.cache.Tier == "pro"
	}

	return false
}

// GetTier returns the current tier
func (v *Validator) GetTier() string {
	if v.cache != nil {
		return v.cache.Tier
	}
	return "free"
}

// GetExpiry returns the license expiry time
func (v *Validator) GetExpiry() *time.Time {
	if v.cache != nil {
		return &v.cache.ExpiresAt
	}
	return nil
}

// GetFirstSeen returns when the user first used ContextSync
func (v *Validator) GetFirstSeen() time.Time {
	if v.cache != nil && !v.cache.FirstSeenAt.IsZero() {
		return v.cache.FirstSeenAt
	}
	return time.Now()
}

// GetSubscriptionType returns the subscription type
func (v *Validator) GetSubscriptionType() SubscriptionType {
	if v.cache != nil && v.cache.SubscriptionID != "" {
		return SubscriptionType(v.cache.SubscriptionID)
	}
	return ""
}

// GetSubscriptionDisplayName returns a human-readable subscription name
func (v *Validator) GetSubscriptionDisplayName() string {
	subType := v.GetSubscriptionType()
	switch subType {
	case SubscriptionMonthly:
		return "Monthly"
	case SubscriptionQuarterly:
		return "Quarterly"
	case SubscriptionYearly:
		return "Yearly"
	default:
		return ""
	}
}

// IsTrialExpired checks if the trial period has expired
func (v *Validator) IsTrialExpired() bool {
	if v.IsPro() {
		return false
	}

	firstSeen := v.GetFirstSeen()
	trialEnd := firstSeen.AddDate(0, 0, TrialDays)
	return time.Now().After(trialEnd)
}

// GetTrialDaysLeft returns days left in trial (negative if expired)
func (v *Validator) GetTrialDaysLeft() int {
	if v.IsPro() {
		return 999 // Unlimited
	}

	firstSeen := v.GetFirstSeen()
	trialEnd := firstSeen.AddDate(0, 0, TrialDays)
	daysLeft := int(time.Until(trialEnd).Hours() / 24)
	return daysLeft
}

// GetFeatures returns the feature set for the current tier
func (v *Validator) GetFeatures() Features {
	trialExpired := v.IsTrialExpired()

	if v.IsPro() {
		return Features{
			MaxTools:        999, // Unlimited
			MemoryRetention: "permanent",
			CanSync:         true,
			CanSaveMemory:   true,
			TrialExpired:    false,
		}
	}

	return Features{
		MaxTools:        2,
		MemoryRetention: "14 days",
		CanSync:         false,
		CanSaveMemory:   false,
		TrialExpired:    trialExpired,
	}
}

// CanUseTool checks if the user can use another tool slot
func (v *Validator) CanUseTool(currentCount int) bool {
	features := v.GetFeatures()
	return currentCount < features.MaxTools
}

// GetMaxTools returns the maximum number of tools allowed
func (v *Validator) GetMaxTools() int {
	if v.IsPro() {
		return 999
	}
	return 2
}

// Activate activates a license key
func (v *Validator) Activate(licenseKey string) error {
	if v.serverURL == "" {
		return fmt.Errorf("server URL not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body := map[string]string{
		"license_key": licenseKey,
		"device_id":   "local", // TODO: get from config
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST",
		v.serverURL+"/api/v1/activate",
		bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("activation failed: status %d", resp.StatusCode)
	}

	var status LicenseStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return err
	}

	// Update local cache and database
	v.mu.Lock()
	v.cache = &LicenseCache{
		Tier:           status.Tier,
		SubscriptionID: status.SubscriptionID,
		Valid:          status.Valid,
		ExpiresAt:      status.ExpiresAt,
		CachedAt:       time.Now(),
	}
	v.mu.Unlock()

	// Update database
	if v.db != nil {
		v.db.Exec(`
			UPDATE license SET
				tier = ?,
				license_key = ?,
				subscription_type = ?,
				valid_until = ?,
				updated_at = datetime('now')
			WHERE id = 1
		`, status.Tier, licenseKey, status.SubscriptionID, status.ExpiresAt.Format(time.RFC3339))
	}

	return nil
}

// Deactivate deactivates the current license
func (v *Validator) Deactivate() {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Keep first_seen_at, just reset tier
	firstSeen := v.cache.FirstSeenAt
	v.cache = &LicenseCache{
		Tier:        "free",
		Valid:       true,
		FirstSeenAt: firstSeen,
		CachedAt:    time.Now(),
	}

	if v.db != nil {
		v.db.Exec(`
			UPDATE license SET
				tier = 'free',
				license_key = NULL,
				subscription_type = NULL,
				valid_until = NULL,
				updated_at = datetime('now')
			WHERE id = 1
		`)
	}
}

func (v *Validator) validateWithServer(ctx context.Context) (*LicenseStatus, error) {
	// Get license key from database
	var licenseKey string
	if v.db != nil {
		v.db.QueryRow("SELECT license_key FROM license WHERE id = 1").Scan(&licenseKey)
	}

	if licenseKey == "" {
		return &LicenseStatus{Tier: "free", Valid: true}, nil
	}

	body := map[string]string{
		"license_key": licenseKey,
		"device_id":   "local",
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST",
		v.serverURL+"/api/v1/validate",
		bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status LicenseStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}
