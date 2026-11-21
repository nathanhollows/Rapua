package services

import (
	"context"
	"sync"
	"time"
)

// stationCacheTTL is how stale a facilitator's page may be. The page asks again
// every 30 seconds from every open window, so this is what keeps a shared link
// from costing the game a full read of every team each time.
const stationCacheTTL = 10 * time.Second

// stationReadTimeout bounds a shared read, which no single request can cancel.
const stationReadTimeout = 30 * time.Second

// stationCache holds one quest's station report for a few seconds, and lets
// simultaneous callers share a single read of it. A link needs no login, so how
// often the page is loaded is not something the game controls.
type stationCache struct {
	mu      sync.Mutex
	entries map[string]*stationCacheEntry
	ttl     time.Duration
}

type stationCacheEntry struct {
	// mu is held while the report is read, so callers arriving meanwhile wait for
	// it and take its answer rather than each starting their own read.
	mu     sync.Mutex
	at     time.Time
	report StationReport
}

func newStationCache() *stationCache {
	return &stationCache{entries: make(map[string]*stationCacheEntry), ttl: stationCacheTTL}
}

func (c *stationCache) entry(questID string, now time.Time) *stationCacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[questID]
	if !ok {
		c.evictStale(now)
		entry = &stationCacheEntry{}
		c.entries[questID] = entry
	}
	return entry
}

// stationCacheKeep is how long a quest's report is kept after it was last read,
// so a quest nobody has looked at for a while stops taking up memory.
const stationCacheKeep = time.Hour

// evictStale drops quests not read lately. It leaves alone any entry being read
// at this moment, which TryLock tells apart without waiting for it.
func (c *stationCache) evictStale(now time.Time) {
	for questID, entry := range c.entries {
		if !entry.mu.TryLock() {
			continue
		}
		if !entry.at.IsZero() && now.Sub(entry.at) > stationCacheKeep {
			delete(c.entries, questID)
		}
		entry.mu.Unlock()
	}
}

// get returns the quest's report, reading it only when the last one is older than
// the cache allows. A failed read is never kept: the next caller tries again.
func (c *stationCache) get(
	questID string, now time.Time, load func(now time.Time) (StationReport, error),
) (StationReport, error) {
	entry := c.entry(questID, now)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	if !entry.at.IsZero() && now.Sub(entry.at) < c.ttl {
		return entry.report, nil
	}
	report, err := load(now)
	if err != nil {
		return StationReport{}, err
	}
	entry.report, entry.at = report, now
	return report, nil
}

// CachedStations is ReadStations for callers that cannot be trusted to ask
// sparingly, such as a page anyone with a link can leave open. The report may be
// a few seconds old.
func (s *RunService) CachedStations(ctx context.Context, questID string) (StationReport, error) {
	// The read is shared by everyone waiting on it, so it must not die with the one
	// request that happened to start it: that request giving up would fail the rest.
	shared, cancel := context.WithTimeout(context.WithoutCancel(ctx), stationReadTimeout)
	defer cancel()
	load := func(now time.Time) (StationReport, error) { return s.ReadStations(shared, questID, now) }
	if s.stations == nil {
		return load(time.Now())
	}
	return s.stations.get(questID, time.Now(), load)
}
