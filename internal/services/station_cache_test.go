package services

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func counting(report StationReport) (func(time.Time) (StationReport, error), *atomic.Int32) {
	var calls atomic.Int32
	return func(time.Time) (StationReport, error) {
		calls.Add(1)
		return report, nil
	}, &calls
}

// A page anyone with a link can leave open must not cost a full read of every
// team each time it asks.
func TestStationCache_ReadsOnceWithinTheWindow(t *testing.T) {
	cache := newStationCache()
	load, calls := counting(StationReport{Playing: 7})
	start := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)

	first, err := cache.get("q", start, load)
	require.NoError(t, err)
	second, err := cache.get("q", start.Add(stationCacheTTL-time.Second), load)
	require.NoError(t, err)

	assert.Equal(t, 7, first.Playing)
	assert.Equal(t, first, second)
	assert.Equal(t, int32(1), calls.Load(), "the second caller took the first's answer")
}

func TestStationCache_ReadsAgainOnceItIsStale(t *testing.T) {
	cache := newStationCache()
	load, calls := counting(StationReport{})
	start := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)

	_, err := cache.get("q", start, load)
	require.NoError(t, err)
	_, err = cache.get("q", start.Add(stationCacheTTL), load)
	require.NoError(t, err)

	assert.Equal(t, int32(2), calls.Load())
}

// One quest's read must never be served for another's.
func TestStationCache_KeepsQuestsApart(t *testing.T) {
	cache := newStationCache()
	now := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)

	a, err := cache.get("a", now, func(time.Time) (StationReport, error) { return StationReport{Playing: 1}, nil })
	require.NoError(t, err)
	b, err := cache.get("b", now, func(time.Time) (StationReport, error) { return StationReport{Playing: 2}, nil })
	require.NoError(t, err)

	assert.Equal(t, 1, a.Playing)
	assert.Equal(t, 2, b.Playing)
}

// A failed read is not kept for ten seconds: the next caller tries again.
func TestStationCache_DoesNotRememberAFailure(t *testing.T) {
	cache := newStationCache()
	now := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)

	_, err := cache.get("q", now, func(time.Time) (StationReport, error) { return StationReport{}, errors.New("down") })
	require.Error(t, err)

	report, err := cache.get("q", now, func(time.Time) (StationReport, error) { return StationReport{Playing: 3}, nil })
	require.NoError(t, err)
	assert.Equal(t, 3, report.Playing)
}

// Many windows asking at once share one read: the point of the cache is the
// moment a shared link is opened by a crowd.
func TestStationCache_ConcurrentCallersShareOneRead(t *testing.T) {
	cache := newStationCache()
	now := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)
	read, calls := counting(StationReport{Playing: 5})
	load := func(at time.Time) (StationReport, error) {
		time.Sleep(20 * time.Millisecond)
		return read(at)
	}

	var wg sync.WaitGroup
	for range 25 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			report, err := cache.get("q", now, load)
			assert.NoError(t, err)
			assert.Equal(t, 5, report.Playing)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), calls.Load())
}

// A quest nobody has read for a long while stops taking up memory, and one being
// read right now is left alone.
func TestStationCache_ForgetsQuestsNobodyReadsAnymore(t *testing.T) {
	cache := newStationCache()
	start := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)
	load, _ := counting(StationReport{})

	for _, id := range []string{"old-1", "old-2"} {
		_, err := cache.get(id, start, load)
		require.NoError(t, err)
	}
	busy := cache.entry("busy", start)
	busy.mu.Lock() // being read at this moment
	busy.at = start

	// A new quest arrives two hours on.
	_, err := cache.get("new", start.Add(2*time.Hour), load)
	require.NoError(t, err)
	busy.mu.Unlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()
	assert.NotContains(t, cache.entries, "old-1")
	assert.NotContains(t, cache.entries, "old-2")
	assert.Contains(t, cache.entries, "new")
	assert.Contains(t, cache.entries, "busy", "an entry mid-read is never pulled out from under it")
}
