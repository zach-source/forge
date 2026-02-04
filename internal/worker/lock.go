package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Lock represents a resource lock for single-threaded roles.
type Lock struct {
	Resource   string    `yaml:"resource"`
	HolderID   string    `yaml:"holder_id"`
	HolderName string    `yaml:"holder_name,omitempty"`
	AcquiredAt time.Time `yaml:"acquired_at"`
}

// locksDir is the directory for lock files (can be overridden for testing).
var locksDir string

// DefaultLocksDir returns the default directory for lock files.
func DefaultLocksDir() string {
	if locksDir != "" {
		return locksDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "workers", "locks")
}

// SetLocksDir sets the locks directory (for testing).
func SetLocksDir(dir string) {
	locksDir = dir
}

// lockPath returns the path to a lock file.
func lockPath(resource string) string {
	return filepath.Join(DefaultLocksDir(), resource+".lock")
}

// AcquireLock attempts to acquire a lock for a resource.
func AcquireLock(resource, workerID string) error {
	path := lockPath(resource)

	// Check if lock already exists
	if lock, err := readLock(path); err == nil {
		if lock.HolderID == workerID {
			// Already held by this worker
			return nil
		}
		return fmt.Errorf("resource %s is locked by %s", resource, lock.HolderID)
	}

	// Create lock
	lock := &Lock{
		Resource:   resource,
		HolderID:   workerID,
		AcquiredAt: time.Now(),
	}

	return writeLock(path, lock)
}

// ReleaseLock releases a lock held by the specified worker.
func ReleaseLock(resource, workerID string) error {
	path := lockPath(resource)

	lock, err := readLock(path)
	if err != nil {
		// Lock doesn't exist, nothing to release
		return nil
	}

	if lock.HolderID != workerID {
		return fmt.Errorf("lock not held by worker %s", workerID)
	}

	return os.Remove(path)
}

// IsLocked checks if a resource is locked and returns the holder ID.
func IsLocked(resource string) (bool, string) {
	path := lockPath(resource)
	lock, err := readLock(path)
	if err != nil {
		return false, ""
	}
	return true, lock.HolderID
}

// GetLock returns the lock for a resource, or nil if not locked.
func GetLock(resource string) *Lock {
	path := lockPath(resource)
	lock, err := readLock(path)
	if err != nil {
		return nil
	}
	return lock
}

// ListLocks returns all active locks.
func ListLocks() ([]*Lock, error) {
	dir := DefaultLocksDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var locks []*Lock
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".lock" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		lock, err := readLock(path)
		if err != nil {
			continue
		}
		locks = append(locks, lock)
	}

	return locks, nil
}

// ForceReleaseLock releases a lock regardless of holder.
func ForceReleaseLock(resource string) error {
	path := lockPath(resource)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CleanStaleLocks removes locks older than the specified duration.
func CleanStaleLocks(maxAge time.Duration) error {
	locks, err := ListLocks()
	if err != nil {
		return err
	}

	for _, lock := range locks {
		if time.Since(lock.AcquiredAt) > maxAge {
			if err := ForceReleaseLock(lock.Resource); err != nil {
				return err
			}
		}
	}

	return nil
}

// readLock reads a lock file.
func readLock(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lock Lock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	return &lock, nil
}

// writeLock writes a lock file atomically.
func writeLock(path string, lock *Lock) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating locks directory: %w", err)
	}

	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}

	// Write atomically
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// ResourceLocks defines the available lockable resources.
var ResourceLocks = []string{
	"merge",  // Merge leader lock
	"deploy", // Deploy leader lock
}

// ValidResource checks if a resource is a valid lockable resource.
func ValidResource(resource string) bool {
	for _, r := range ResourceLocks {
		if r == resource {
			return true
		}
	}
	return false
}
