package service

import (
	"codechunking/internal/domain/entity"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// deduplicationEntry represents a tracked alert for deduplication.
type deduplicationEntry struct {
	key        string
	firstSeen  time.Time
	lastSeen   time.Time
	count      int
	suppressed bool
	alertData  *entity.Alert // Keep reference for statistics
}

// smartAlertDeduplicator implements SmartAlertDeduplicator interface.
type smartAlertDeduplicator struct {
	deduplicationWindow time.Duration
	entries             map[string]*deduplicationEntry
	totalProcessed      int64
	duplicatesDetected  int64
	alertsSuppressed    int64
	mu                  sync.RWMutex
}

// NewSmartAlertDeduplicator creates a new alert deduplicator with the specified time window.
func NewSmartAlertDeduplicator(deduplicationWindow time.Duration) SmartAlertDeduplicator {
	return &smartAlertDeduplicator{
		deduplicationWindow: deduplicationWindow,
		entries:             make(map[string]*deduplicationEntry),
	}
}

// IsDuplicate checks if an alert is a duplicate within the deduplication window.
func (d *smartAlertDeduplicator) IsDuplicate(alert *entity.Alert) (bool, error) {
	if alert == nil {
		return false, errors.New("alert cannot be nil")
	}

	key := d.generateDeduplicationKey(alert)

	d.mu.RLock()
	entry, exists := d.entries[key]
	d.mu.RUnlock()

	if !exists {
		return false, nil
	}

	// Check if entry is still within deduplication window
	if time.Since(entry.firstSeen) > d.deduplicationWindow {
		// Entry expired, clean it up
		d.mu.Lock()
		delete(d.entries, key)
		d.mu.Unlock()
		return false, nil
	}

	return true, nil
}

// RecordAlert records an alert in the deduplication system.
func (d *smartAlertDeduplicator) RecordAlert(alertType, message string) {
	if alertType == "" || message == "" {
		return // Invalid input, ignore
	}

	// Generate key from the parameters
	key := fmt.Sprintf("%s:%s", alertType, message)
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	d.totalProcessed++

	if entry, exists := d.entries[key]; exists {
		// Update existing entry
		entry.lastSeen = now
		entry.count++
		d.duplicatesDetected++
	} else {
		// Create new entry
		d.entries[key] = &deduplicationEntry{
			key:       key,
			firstSeen: now,
			lastSeen:  now,
			count:     1,
			alertData: nil, // No alert data for string-based alerts
		}
	}
}

// TrackDuplicate tracks that a duplicate alert was encountered.
func (d *smartAlertDeduplicator) TrackDuplicate(alert *entity.Alert) {
	if alert == nil {
		return
	}

	key := d.generateDeduplicationKey(alert)

	d.mu.Lock()
	defer d.mu.Unlock()

	if entry, exists := d.entries[key]; exists {
		entry.count++
		entry.lastSeen = time.Now()
		d.duplicatesDetected++
	}
}

// GetDuplicateCount returns the number of duplicates for a specific alert key.
func (d *smartAlertDeduplicator) GetDuplicateCount(deduplicationKey string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if entry, exists := d.entries[deduplicationKey]; exists {
		return entry.count
	}

	return 0
}

// ShouldSuppress determines if an alert should be suppressed due to excessive duplicates.
func (d *smartAlertDeduplicator) ShouldSuppress(deduplicationKey string, threshold int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, exists := d.entries[deduplicationKey]
	if !exists {
		return false
	}

	if entry.count >= threshold && !entry.suppressed {
		entry.suppressed = true
		d.alertsSuppressed++
		return true
	}

	return entry.suppressed
}

// CleanExpiredEntries removes entries older than the deduplication window.
func (d *smartAlertDeduplicator) CleanExpiredEntries() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, entry := range d.entries {
		if now.Sub(entry.firstSeen) > d.deduplicationWindow {
			delete(d.entries, key)
			removed++
		}
	}

	return removed
}

// GetActiveKeys returns all currently tracked deduplication keys.
func (d *smartAlertDeduplicator) GetActiveKeys() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	keys := make([]string, 0, len(d.entries))
	for key := range d.entries {
		keys = append(keys, key)
	}

	return keys
}

// GetWindowSize returns the configured deduplication time window.
func (d *smartAlertDeduplicator) GetWindowSize() time.Duration {
	return d.deduplicationWindow
}

// GetMemoryUsage returns approximate memory usage in bytes.
func (d *smartAlertDeduplicator) GetMemoryUsage() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Base memory for the struct
	baseMemory := int64(unsafe.Sizeof(*d))

	// Memory for map entries
	entryMemory := int64(0)
	for key, entry := range d.entries {
		// Key string memory
		entryMemory += int64(len(key))
		// Entry struct memory
		entryMemory += int64(unsafe.Sizeof(*entry))
		// Rough estimate for alert data
		if entry.alertData != nil {
			entryMemory += 512 // Conservative estimate per alert
		}
	}

	// Map overhead estimation
	mapOverhead := int64(len(d.entries)) * 8 // Rough estimate for map bucket overhead

	return baseMemory + entryMemory + mapOverhead
}

// GetStatistics returns comprehensive deduplication statistics.
func (d *smartAlertDeduplicator) GetStatistics() DeduplicationStatistics {
	d.mu.RLock()
	defer d.mu.RUnlock()

	keyDistribution := make(map[string]int)
	hourlyDuplicates := make(map[int]int64)

	var oldestAge time.Duration
	var totalKeyLifetime time.Duration
	now := time.Now()

	for _, entry := range d.entries {
		// Key distribution by alert type or component
		if entry.alertData != nil {
			keyType := fmt.Sprintf("severity_%s", entry.alertData.Severity().String())
			keyDistribution[keyType] += entry.count
		}

		// Hourly duplicate counts (simple hour-based grouping)
		hour := entry.lastSeen.Hour()
		hourlyDuplicates[hour] += int64(entry.count - 1) // Exclude original

		// Age calculations
		age := now.Sub(entry.firstSeen)
		if age > oldestAge {
			oldestAge = age
		}
		totalKeyLifetime += age
	}

	var averageKeyLifetime time.Duration
	if len(d.entries) > 0 {
		averageKeyLifetime = totalKeyLifetime / time.Duration(len(d.entries))
	}

	var deduplicationRate, suppressionRate float64
	if d.totalProcessed > 0 {
		deduplicationRate = float64(d.duplicatesDetected) / float64(d.totalProcessed)
		suppressionRate = float64(d.alertsSuppressed) / float64(d.totalProcessed)
	}

	return DeduplicationStatistics{
		TotalAlertsProcessed:  d.totalProcessed,
		DuplicatesDetected:    d.duplicatesDetected,
		AlertsSuppressed:      d.alertsSuppressed,
		ActiveKeys:            len(d.entries),
		MemoryUsageBytes:      d.GetMemoryUsage(),
		OldestEntryAge:        oldestAge,
		AverageKeyLifetime:    averageKeyLifetime,
		DeduplicationRate:     deduplicationRate,
		SuppressionRate:       suppressionRate,
		KeyDistribution:       keyDistribution,
		HourlyDuplicateCounts: hourlyDuplicates,
	}
}

// generateDeduplicationKey creates a unique key for alert deduplication.
func (d *smartAlertDeduplicator) generateDeduplicationKey(alert *entity.Alert) string {
	// Create a hash based on alert characteristics that make it unique
	// for deduplication purposes (type, component, basic message pattern)

	data := fmt.Sprintf("%s|%s|%s",
		alert.Type().String(),
		alert.Severity().String(),
		alert.ErrorID(),
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for reasonable key length
}

// ShouldSendAlert determines if an alert should be sent based on deduplication rules.
func (d *smartAlertDeduplicator) ShouldSendAlert(alertType, message string) bool {
	if alertType == "" || message == "" {
		return false // Invalid input, don't send
	}

	// Generate key from the parameters
	key := fmt.Sprintf("%s:%s", alertType, message)
	now := time.Now()

	d.mu.RLock()
	entry, exists := d.entries[key]
	d.mu.RUnlock()

	if !exists {
		// First time seeing this alert, should send it
		return true
	}

	// Check if entry is still within deduplication window
	if now.Sub(entry.firstSeen) > d.deduplicationWindow {
		// Entry expired, should send the alert
		return true
	}

	// Check if it's suppressed due to too many duplicates
	if entry.suppressed {
		return false
	}

	// Within deduplication window, don't send duplicate
	return false
}

// Reset resets the deduplicator state, clearing all tracked entries and statistics.
func (d *smartAlertDeduplicator) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Clear all entries
	d.entries = make(map[string]*deduplicationEntry)

	// Reset statistics
	d.totalProcessed = 0
	d.duplicatesDetected = 0
	d.alertsSuppressed = 0
}
