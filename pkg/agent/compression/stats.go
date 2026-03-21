package compression

import (
	"sync"
	"time"
)

var (
	sessionStats = make(map[string]*Stats)
	statsMu      sync.RWMutex
)

// RecordCompression stores compression result for a session
// Thread-safe using mutex (inspired by crush's csync pattern)
func RecordCompression(sessionKey string, result *Result) {
	statsMu.Lock()
	defer statsMu.Unlock()
	
	stats, exists := sessionStats[sessionKey]
	if !exists {
		stats = &Stats{}
		sessionStats[sessionKey] = stats
	}
	
	stats.LastCompression = result
	stats.LastCompressionTime = time.Now()
	stats.TotalCompressions++
	
	if result != nil {
		stats.TotalTokensSaved += result.OriginalTokenCount - result.CompressedTokenCount
	}
}

// GetStats retrieves compression stats for a session
func GetStats(sessionKey string) *Stats {
	statsMu.RLock()
	defer statsMu.RUnlock()
	return sessionStats[sessionKey]
}

// ClearStats removes compression stats for a session
func ClearStats(sessionKey string) {
	statsMu.Lock()
	defer statsMu.Unlock()
	delete(sessionStats, sessionKey)
}

// GetAllStats returns a copy of all session stats (for debugging/admin)
func GetAllStats() map[string]*Stats {
	statsMu.RLock()
	defer statsMu.RUnlock()
	
	result := make(map[string]*Stats, len(sessionStats))
	for k, v := range sessionStats {
		result[k] = v
	}
	return result
}
