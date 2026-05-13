package license

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"contextsync/internal/db"
)

const (
	TrialDays        = 14
	SignatureMaxAge  = 7 * 24 * time.Hour // Signature valid for 7 days offline
)

// Embedded public key for signature verification
// This should be replaced with the actual public key from the server
var embeddedPublicKey ed25519.PublicKey

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
	// Signature fields
	Signature string    `json:"signature"`
	SignedAt  time.Time `json:"signed_at"`
	AccountID string    `json:"account_id"`
}

type LicenseStatus struct {
	Valid          bool      `json:"valid"`
	Tier           string    `json:"tier"`
	SubscriptionID string    `json:"subscription_id"`
	ExpiresAt      time.Time `json:"expires_at"`
	// Signature fields
	Signature string `json:"signature"`
	SignedAt  string `json:"signed_at"`
	AccountID string `json:"account_id"`
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

// SetPublicKey sets the embedded public key for signature verification
func SetPublicKey(pubKeyBase64 string) error {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: %d", len(pubKeyBytes))
	}
	embeddedPublicKey = ed25519.PublicKey(pubKeyBytes)
	return nil
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
	var validUntil, firstSeenAt, subscriptionType, signature, signedAt, accountID sql.NullString

	err := v.db.QueryRow(`
		SELECT tier, license_key, valid_until, first_seen_at, subscription_type,
		       signature, signed_at, account_id
		FROM license WHERE id = 1
	`).Scan(&tier, &licenseKey, &validUntil, &firstSeenAt, &subscriptionType,
		&signature, &signedAt, &accountID)

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
		if signature.Valid {
			v.cache.Signature = signature.String
		}
		if signedAt.Valid {
			v.cache.SignedAt, _ = time.Parse(time.RFC3339, signedAt.String)
		}
		if accountID.Valid {
			v.cache.AccountID = accountID.String
		}
	}
}

// verifySignature verifies the license signature
func (v *Validator) verifySignature(tier, expiresAt, accountID, signature string) bool {
	if embeddedPublicKey == nil || signature == "" {
		return false
	}

	// Build the data that was signed
	data := fmt.Sprintf("%s|%s|%s", tier, expiresAt, accountID)

	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	// Verify with Ed25519
	return ed25519.Verify(embeddedPublicKey, []byte(data), sigBytes)
}

// IsPro checks if the user has a valid Pro license
func (v *Validator) IsPro() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Check cache first (valid for 24 hours)
	if v.cache != nil && time.Since(v.cache.CachedAt) < 24*time.Hour {
		return v.isProFromCache()
	}

	// Validate with server
	if v.serverURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		status, err := v.validateWithServer(ctx)
		if err == nil {
			v.updateCacheFromServer(status)
			return status.Tier == "pro" && status.Valid
		}
	}

	// Fall back to cached value with signature verification
	return v.isProFromCache()
}

// isProFromCache checks Pro status from cache with signature verification
func (v *Validator) isProFromCache() bool {
	if v.cache == nil {
		return false
	}

	// Free tier doesn't need signature
	if v.cache.Tier != "pro" {
		return false
	}

	// For Pro tier, require valid signature
	if v.cache.Signature == "" {
		return false
	}

	// Check signature age
	if !v.cache.SignedAt.IsZero() && time.Since(v.cache.SignedAt) > SignatureMaxAge {
		return false // Signature too old
	}

	// Verify signature
	expiresAt := ""
	if !v.cache.ExpiresAt.IsZero() {
		expiresAt = v.cache.ExpiresAt.Format(time.RFC3339)
	}

	if !v.verifySignature(v.cache.Tier, expiresAt, v.cache.AccountID, v.cache.Signature) {
		return false // Invalid signature
	}

	return v.cache.Valid
}

// updateCacheFromServer updates the cache from server response
func (v *Validator) updateCacheFromServer(status *LicenseStatus) {
	v.cache = &LicenseCache{
		Tier:           status.Tier,
		SubscriptionID: status.SubscriptionID,
		Valid:          status.Valid,
		ExpiresAt:      status.ExpiresAt,
		CachedAt:       time.Now(),
		Signature:      status.Signature,
		AccountID:      status.AccountID,
	}
	if status.SignedAt != "" {
		v.cache.SignedAt, _ = time.Parse(time.RFC3339, status.SignedAt)
	}

	// Update database
	if v.db != nil && status.Tier == "pro" {
		v.db.Exec(`
			UPDATE license SET
				tier = ?,
				subscription_type = ?,
				valid_until = ?,
				signature = ?,
				signed_at = ?,
				account_id = ?,
				updated_at = datetime('now')
			WHERE id = 1
		`, status.Tier, status.SubscriptionID, status.ExpiresAt.Format(time.RFC3339),
			status.Signature, status.SignedAt, status.AccountID)
	}
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
	v.updateCacheFromServer(&status)
	v.mu.Unlock()

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
				signature = NULL,
				signed_at = NULL,
				account_id = NULL,
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
