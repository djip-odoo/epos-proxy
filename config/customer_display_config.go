package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CustomerDisplayURL holds configuration for a single customer display URL.
type CustomerDisplayURL struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var (
	cdConfigPath string
	cdConfigOnce sync.Once

	cdURLs   []CustomerDisplayURL
	cdURLsMu sync.RWMutex
)

func initCDConfigPath() {
	cdConfigOnce.Do(func() {
		base, err := os.UserConfigDir()
		if err != nil {
			return
		}
		cdConfigPath = filepath.Join(base, AppName, "customer_display.json")
	})
}

func ensureCDURLsLoaded() error {
	initCDConfigPath()

	cdURLsMu.RLock()
	loaded := cdURLs != nil
	cdURLsMu.RUnlock()

	if loaded {
		return nil
	}

	cdURLsMu.Lock()
	defer cdURLsMu.Unlock()

	if cdURLs != nil {
		return nil
	}

	return loadCDURLsLocked()
}

func loadCDURLsLocked() error {
	data, err := os.ReadFile(cdConfigPath)
	if os.IsNotExist(err) {
		cdURLs = []CustomerDisplayURL{}
		return saveCDURLsLocked()
	}
	if err != nil {
		return fmt.Errorf("customer display config read error: %w", err)
	}

	if err := json.Unmarshal(data, &cdURLs); err != nil {
		cdURLs = []CustomerDisplayURL{}
		return saveCDURLsLocked()
	}

	return nil
}

func saveCDURLsLocked() error {
	if err := os.MkdirAll(filepath.Dir(cdConfigPath), 0755); err != nil {
		return fmt.Errorf("cannot create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cdURLs, "", "  ")
	if err != nil {
		return fmt.Errorf("customer display config marshal error: %w", err)
	}

	tmpPath := cdConfigPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err == nil {
		if err := os.Rename(tmpPath, cdConfigPath); err == nil {
			return nil
		}
	}

	return os.WriteFile(cdConfigPath, data, 0644)
}

// ValidateCustomerDisplayURL checks that a URL is well-formed HTTP/HTTPS.
func ValidateCustomerDisplayURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("URL is required")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return errors.New("URL must use http:// or https://")
	}

	if parsed.Host == "" {
		return errors.New("URL must include a host")
	}

	return nil
}

// GetCustomerDisplayURLs returns a copy of all configured URLs.
func GetCustomerDisplayURLs() ([]CustomerDisplayURL, error) {
	if err := ensureCDURLsLoaded(); err != nil {
		return nil, err
	}

	cdURLsMu.RLock()
	defer cdURLsMu.RUnlock()

	result := make([]CustomerDisplayURL, len(cdURLs))
	copy(result, cdURLs)
	return result, nil
}

// GetActiveCustomerDisplayURL returns the first enabled URL, or nil if none.
func GetActiveCustomerDisplayURL() (*CustomerDisplayURL, error) {
	if err := ensureCDURLsLoaded(); err != nil {
		return nil, err
	}

	cdURLsMu.RLock()
	defer cdURLsMu.RUnlock()

	for i := range cdURLs {
		if cdURLs[i].Enabled {
			cp := cdURLs[i]
			return &cp, nil
		}
	}
	return nil, nil
}

// AddCustomerDisplayURL creates a new URL record. Returns the created record.
func AddCustomerDisplayURL(name, rawURL, description string) (CustomerDisplayURL, error) {
	if strings.TrimSpace(name) == "" {
		return CustomerDisplayURL{}, errors.New("name is required")
	}

	if err := ValidateCustomerDisplayURL(rawURL); err != nil {
		return CustomerDisplayURL{}, err
	}

	if err := ensureCDURLsLoaded(); err != nil {
		return CustomerDisplayURL{}, err
	}

	cdURLsMu.Lock()
	defer cdURLsMu.Unlock()

	now := time.Now()
	record := CustomerDisplayURL{
		ID:          uuid.NewString(),
		Name:        strings.TrimSpace(name),
		URL:         rawURL,
		Description: strings.TrimSpace(description),
		Enabled:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	cdURLs = append(cdURLs, record)
	if err := saveCDURLsLocked(); err != nil {
		return CustomerDisplayURL{}, err
	}

	return record, nil
}

// UpdateCustomerDisplayURL updates an existing record by ID.
// If enabled is true, all other records are disabled (only one active at a time).
func UpdateCustomerDisplayURL(id, name, rawURL, description string, enabled bool) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}

	if err := ValidateCustomerDisplayURL(rawURL); err != nil {
		return err
	}

	if err := ensureCDURLsLoaded(); err != nil {
		return err
	}

	cdURLsMu.Lock()
	defer cdURLsMu.Unlock()

	idx := -1
	for i := range cdURLs {
		if cdURLs[i].ID == id {
			idx = i
			break
		}
	}

	if idx < 0 {
		return fmt.Errorf("customer display URL %q not found", id)
	}

	// If enabling this record, disable all others.
	if enabled {
		for i := range cdURLs {
			cdURLs[i].Enabled = false
		}
	}

	cdURLs[idx].Name = strings.TrimSpace(name)
	cdURLs[idx].URL = rawURL
	cdURLs[idx].Description = strings.TrimSpace(description)
	cdURLs[idx].Enabled = enabled
	cdURLs[idx].UpdatedAt = time.Now()

	return saveCDURLsLocked()
}

// SetActiveCustomerDisplayURL marks the given ID as enabled and all others disabled.
func SetActiveCustomerDisplayURL(id string) error {
	if err := ensureCDURLsLoaded(); err != nil {
		return err
	}

	cdURLsMu.Lock()
	defer cdURLsMu.Unlock()

	found := false
	for i := range cdURLs {
		if cdURLs[i].ID == id {
			cdURLs[i].Enabled = true
			cdURLs[i].UpdatedAt = time.Now()
			found = true
		} else {
			cdURLs[i].Enabled = false
		}
	}

	if !found {
		return fmt.Errorf("customer display URL %q not found", id)
	}

	return saveCDURLsLocked()
}

// DisableCustomerDisplayURL disables the given URL without deleting it.
func DisableCustomerDisplayURL(id string) error {
	if err := ensureCDURLsLoaded(); err != nil {
		return err
	}

	cdURLsMu.Lock()
	defer cdURLsMu.Unlock()

	for i := range cdURLs {
		if cdURLs[i].ID == id {
			cdURLs[i].Enabled = false
			cdURLs[i].UpdatedAt = time.Now()
			return saveCDURLsLocked()
		}
	}

	return fmt.Errorf("customer display URL %q not found", id)
}

// DeleteCustomerDisplayURL removes a URL record by ID.
func DeleteCustomerDisplayURL(id string) error {
	if err := ensureCDURLsLoaded(); err != nil {
		return err
	}

	cdURLsMu.Lock()
	defer cdURLsMu.Unlock()

	for i, u := range cdURLs {
		if u.ID == id {
			cdURLs = append(cdURLs[:i], cdURLs[i+1:]...)
			return saveCDURLsLocked()
		}
	}

	return fmt.Errorf("customer display URL %q not found", id)
}

// ReloadCustomerDisplayConfig forces a reload from disk on next access.
// Useful for testing or after external file edits.
func ReloadCustomerDisplayConfig() {
	cdURLsMu.Lock()
	defer cdURLsMu.Unlock()
	cdURLs = nil
}
