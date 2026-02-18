package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BlacklistEntry represents a user-defined blacklist rule
type BlacklistEntry struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`       // "pid", "path", "pattern"
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

// builtinBlacklistedPaths contains system paths that are always blacklisted
var builtinBlacklistedPaths = []string{
	"/usr/libexec/",
	"/usr/lib/",
	"/private/var/",
	"/var/",
	"/opt/X11/",
}

// builtinBlacklistedNames contains executable names that are always blacklisted
var builtinBlacklistedNames = []string{
	"localhost-magic-daemon",
	"localhost-magic",
}

// builtinBlacklistedPatterns contains name patterns that are always blacklisted
var builtinBlacklistedPatterns = []string{
	`^localhost-magic`,
}

// interpreters is the list of known language interpreters
var interpreters = []string{"python", "python3", "node", "nodejs", "ruby", "perl", "php", "java"}

// BlacklistStore manages persistent blacklist entries
type BlacklistStore struct {
	path    string
	entries []*BlacklistEntry
	mu      sync.RWMutex
}

// NewBlacklistStore creates a new BlacklistStore, loading existing entries from disk
func NewBlacklistStore(path string) (*BlacklistStore, error) {
	bs := &BlacklistStore{
		path:    path,
		entries: make([]*BlacklistEntry, 0),
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Load existing entries
	if err := bs.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load blacklist: %w", err)
	}

	return bs, nil
}

// Add creates a new blacklist entry and persists it
func (bs *BlacklistStore) Add(entryType, value string) (*BlacklistEntry, error) {
	// Validate type
	if entryType != "pid" && entryType != "path" && entryType != "pattern" {
		return nil, fmt.Errorf("invalid blacklist type: %s (must be pid, path, or pattern)", entryType)
	}

	// Validate pid is a number
	if entryType == "pid" {
		if _, err := strconv.Atoi(value); err != nil {
			return nil, fmt.Errorf("invalid PID value: %s", value)
		}
	}

	// Validate pattern compiles
	if entryType == "pattern" {
		if _, err := regexp.Compile(value); err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}

	entry := &BlacklistEntry{
		ID:        id,
		Type:      entryType,
		Value:     value,
		CreatedAt: time.Now(),
	}

	bs.mu.Lock()
	bs.entries = append(bs.entries, entry)
	err = bs.persist()
	bs.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to persist blacklist: %w", err)
	}

	return entry, nil
}

// Remove deletes a blacklist entry by ID
func (bs *BlacklistStore) Remove(id string) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	for i, entry := range bs.entries {
		if entry.ID == id {
			bs.entries = append(bs.entries[:i], bs.entries[i+1:]...)
			return bs.persist()
		}
	}

	return fmt.Errorf("blacklist entry not found: %s", id)
}

// List returns all blacklist entries
func (bs *BlacklistStore) List() []*BlacklistEntry {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	result := make([]*BlacklistEntry, len(bs.entries))
	copy(result, bs.entries)
	return result
}

// IsBlacklisted checks if a service should be ignored based on both
// built-in system patterns and user-defined blacklist entries
func (bs *BlacklistStore) IsBlacklisted(exePath string, args []string) bool {
	// Check built-in rules first

	// Check own executable
	if strings.Contains(exePath, "localhost-magic-daemon") || strings.Contains(exePath, "localhost-magic") {
		return true
	}

	// Check built-in blacklisted names
	exeName := filepath.Base(exePath)
	for _, name := range builtinBlacklistedNames {
		if exeName == name {
			return true
		}
	}

	// For interpreted languages, check the script path instead of interpreter
	if isInterpreterName(exeName) && len(args) > 1 {
		scriptPath := args[1]
		if isUserPath(scriptPath) {
			// User script in a user directory -- do not blacklist based on interpreter path
			// but still check user-defined rules below
			goto checkUserRules
		}
	}

	// Check built-in blacklisted paths
	for _, prefix := range builtinBlacklistedPaths {
		if strings.HasPrefix(exePath, prefix) {
			if strings.HasPrefix(exePath, "/Applications/") {
				continue
			}
			return true
		}
	}

checkUserRules:
	// Check user-defined blacklist entries
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	for _, entry := range bs.entries {
		switch entry.Type {
		case "pid":
			// PID-based blacklisting is checked at the caller level
			// since we don't have PID info here; skip
			continue
		case "path":
			if exePath == entry.Value || strings.HasPrefix(exePath, entry.Value) {
				return true
			}
		case "pattern":
			matched, err := regexp.MatchString(entry.Value, exePath)
			if err == nil && matched {
				return true
			}
			// Also check against args joined
			if len(args) > 0 {
				argsJoined := strings.Join(args, " ")
				matched, err = regexp.MatchString(entry.Value, argsJoined)
				if err == nil && matched {
					return true
				}
			}
		}
	}

	return false
}

// IsBlacklistedPID checks if a specific PID is blacklisted by user entries
func (bs *BlacklistStore) IsBlacklistedPID(pid int) bool {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	pidStr := strconv.Itoa(pid)
	for _, entry := range bs.entries {
		if entry.Type == "pid" && entry.Value == pidStr {
			return true
		}
	}
	return false
}

// DefaultBlacklistPath returns the default blacklist storage path
func DefaultBlacklistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "localhost-magic", "blacklist.json")
}

// load reads blacklist entries from disk
func (bs *BlacklistStore) load() error {
	data, err := os.ReadFile(bs.path)
	if err != nil {
		return err
	}

	var entries []*BlacklistEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	bs.entries = entries
	return nil
}

// persist writes blacklist entries to disk atomically
func (bs *BlacklistStore) persist() error {
	data, err := json.MarshalIndent(bs.entries, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file, then rename
	dir := filepath.Dir(bs.path)
	tmpFile, err := os.CreateTemp(dir, "blacklist-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmpFile.Chmod(0666); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, bs.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// generateID creates a random hex ID
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// isInterpreterName checks if the executable is a language interpreter
func isInterpreterName(name string) bool {
	for _, interp := range interpreters {
		if name == interp || strings.HasPrefix(name, interp) {
			return true
		}
	}
	return false
}

// isUserPath checks if a path is in a user directory (not system)
func isUserPath(path string) bool {
	userIndicators := []string{"/home/", "/Users/", "/tmp/", "/var/tmp/"}
	for _, indicator := range userIndicators {
		if strings.HasPrefix(path, indicator) {
			return true
		}
	}
	return false
}
