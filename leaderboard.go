// Package leaderboard provides a concurrency-safe, in-memory leaderboard
// with a fixed maximum number of players.
package leaderboard

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	// ErrInvalidCapacity is returned by New when capacity is not positive.
	ErrInvalidCapacity = errors.New("leaderboard: capacity must be positive")
	// ErrEmptyID is returned when a player ID is empty.
	ErrEmptyID = errors.New("leaderboard: player ID must be non-empty")
	// ErrFull is returned when a new player cannot be added because the
	// leaderboard is at capacity. Existing players can still be updated.
	ErrFull = errors.New("leaderboard: at capacity, new player rejected")
)

// Entry is a single ranked player. Entries are ordered by score
// descending; ties are broken by player ID in ascending lexicographic order.
type Entry struct {
	ID    string
	Score int64
}

// Leaderboard is a fixed-capacity, concurrency-safe ranking of players.
// The zero value is not usable; construct one with New.
type Leaderboard struct {
	mu       sync.RWMutex
	capacity int
	scores   map[string]int64
	sorted   []Entry // kept sorted by rank order at all times
}

// New creates a Leaderboard that holds at most capacity players.
func New(capacity int) (*Leaderboard, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidCapacity, capacity)
	}
	return &Leaderboard{
		capacity: capacity,
		scores:   make(map[string]int64),
		sorted:   make([]Entry, 0, capacity),
	}, nil
}

// before reports whether a ranks strictly ahead of b.
func before(a, b Entry) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	return a.ID < b.ID
}

// search returns the index at which e belongs in rank order.
// Callers must hold the lock.
func (l *Leaderboard) search(e Entry) int {
	return sort.Search(len(l.sorted), func(i int) bool {
		return !before(l.sorted[i], e)
	})
}

// insert places e at its sorted position. Callers must hold the write lock.
func (l *Leaderboard) insert(e Entry) {
	i := l.search(e)
	l.sorted = append(l.sorted, Entry{})
	copy(l.sorted[i+1:], l.sorted[i:])
	l.sorted[i] = e
}

// removeAt deletes the entry at index i. Callers must hold the write lock.
func (l *Leaderboard) removeAt(i int) {
	copy(l.sorted[i:], l.sorted[i+1:])
	l.sorted[len(l.sorted)-1] = Entry{}
	l.sorted = l.sorted[:len(l.sorted)-1]
}

// set assigns score to id, creating the player if needed. It reports
// whether the player already existed. Callers must hold the write lock.
func (l *Leaderboard) set(id string, score int64) error {
	old, ok := l.scores[id]
	if !ok {
		if len(l.sorted) >= l.capacity {
			return ErrFull
		}
	} else {
		l.removeAt(l.search(Entry{ID: id, Score: old}))
	}
	l.scores[id] = score
	l.insert(Entry{ID: id, Score: score})
	return nil
}

// SetScore sets the absolute score of a player, creating the player if
// necessary. It returns ErrFull when a new player would exceed capacity;
// updates to existing players always succeed.
func (l *Leaderboard) SetScore(id string, score int64) error {
	if id == "" {
		return ErrEmptyID
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.set(id, score)
}

// AddScore adds delta to the player's current score, creating the player
// with an initial score of delta if the player does not exist. It returns
// ErrFull when a new player would exceed capacity.
func (l *Leaderboard) AddScore(id string, delta int64) error {
	if id == "" {
		return ErrEmptyID
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.set(id, l.scores[id]+delta)
}

// Remove deletes a player. It reports whether the player existed.
func (l *Leaderboard) Remove(id string) bool {
	if id == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	score, ok := l.scores[id]
	if !ok {
		return false
	}
	delete(l.scores, id)
	l.removeAt(l.search(Entry{ID: id, Score: score}))
	return true
}

// Get returns the player's current score and whether the player exists.
func (l *Leaderboard) Get(id string) (int64, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	score, ok := l.scores[id]
	return score, ok
}

// Rank returns the player's 1-based rank and whether the player exists.
func (l *Leaderboard) Rank(id string) (int, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	score, ok := l.scores[id]
	if !ok {
		return 0, false
	}
	return l.search(Entry{ID: id, Score: score}) + 1, true
}

// Top returns up to n leading entries in rank order. n <= 0 yields an
// empty slice. The result is an independent copy.
func (l *Leaderboard) Top(n int) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if n < 0 {
		n = 0
	}
	if n > len(l.sorted) {
		n = len(l.sorted)
	}
	out := make([]Entry, n)
	copy(out, l.sorted[:n])
	return out
}

// Around returns up to before entries ranked ahead of id, the player
// itself, and up to after entries ranked behind it, in rank order.
// Negative before/after are treated as zero. It reports whether the
// player exists. The result is an independent copy.
func (l *Leaderboard) Around(id string, before, after int) ([]Entry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	score, ok := l.scores[id]
	if !ok {
		return nil, false
	}
	if before < 0 {
		before = 0
	}
	if after < 0 {
		after = 0
	}
	idx := l.search(Entry{ID: id, Score: score})
	start := idx - before
	if start < 0 {
		start = 0
	}
	end := idx + after + 1
	if end > len(l.sorted) {
		end = len(l.sorted)
	}
	out := make([]Entry, end-start)
	copy(out, l.sorted[start:end])
	return out, true
}

// Snapshot returns an independent copy of all entries in rank order.
func (l *Leaderboard) Snapshot() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.sorted))
	copy(out, l.sorted)
	return out
}

// Len returns the current number of players.
func (l *Leaderboard) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.sorted)
}

// Cap returns the maximum number of players.
func (l *Leaderboard) Cap() int {
	return l.capacity
}
