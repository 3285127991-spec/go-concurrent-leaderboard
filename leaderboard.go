// Package leaderboard provides a concurrent, in-memory leaderboard with a
// fixed maximum number of players. Players are ranked by score descending;
// ties are broken by player ID in ascending lexicographic order.
package leaderboard

import (
	"errors"
	"sort"
	"sync"
)

var (
	// ErrInvalidCapacity is returned by New when capacity is not positive.
	ErrInvalidCapacity = errors.New("leaderboard: capacity must be positive")
	// ErrEmptyID is returned when a player ID is empty.
	ErrEmptyID = errors.New("leaderboard: player ID must not be empty")
	// ErrFull is returned when adding a new player would exceed capacity.
	// Updates to existing players are never rejected because of capacity.
	ErrFull = errors.New("leaderboard: capacity reached, cannot add new player")
)

// Player is a snapshot of one leaderboard entry. Rank is 1-based.
type Player struct {
	ID    string
	Score int64
	Rank  int
}

// entry is the internal stored form of a player (rank is positional).
type entry struct {
	id    string
	score int64
}

// Leaderboard is a fixed-capacity, concurrency-safe ranking of players.
// The zero value is not usable; create one with New.
type Leaderboard struct {
	mu       sync.RWMutex
	capacity int
	scores   map[string]int64
	sorted   []entry // sorted by score desc, then id asc
}

// New creates a Leaderboard that holds at most capacity players.
func New(capacity int) (*Leaderboard, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	return &Leaderboard{
		capacity: capacity,
		scores:   make(map[string]int64),
		sorted:   make([]entry, 0, capacity),
	}, nil
}

// Len returns the current number of players.
func (l *Leaderboard) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.sorted)
}

// Capacity returns the maximum number of players.
func (l *Leaderboard) Capacity() int {
	return l.capacity
}

// SetScore sets the absolute score of a player, creating the player if
// needed. It returns ErrFull only when a new player would exceed capacity.
func (l *Leaderboard) SetScore(id string, score int64) error {
	if id == "" {
		return ErrEmptyID
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if old, ok := l.scores[id]; ok {
		l.removeLocked(id, old)
	} else if len(l.sorted) >= l.capacity {
		return ErrFull
	}
	l.insertLocked(entry{id: id, score: score})
	l.scores[id] = score
	return nil
}

// AddScore adds delta to the player's score, creating the player with an
// initial score of delta if it does not exist. It returns ErrFull only when
// a new player would exceed capacity.
func (l *Leaderboard) AddScore(id string, delta int64) error {
	if id == "" {
		return ErrEmptyID
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	old, ok := l.scores[id]
	if ok {
		l.removeLocked(id, old)
	} else if len(l.sorted) >= l.capacity {
		return ErrFull
	}
	score := old + delta
	l.insertLocked(entry{id: id, score: score})
	l.scores[id] = score
	return nil
}

// Remove deletes a player. It reports whether the player existed.
func (l *Leaderboard) Remove(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	score, ok := l.scores[id]
	if !ok {
		return false
	}
	l.removeLocked(id, score)
	delete(l.scores, id)
	return true
}

// Get returns the player with the given ID, including its current rank.
func (l *Leaderboard) Get(id string) (Player, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	score, ok := l.scores[id]
	if !ok {
		return Player{}, false
	}
	idx := l.searchLocked(entry{id: id, score: score})
	return Player{ID: id, Score: score, Rank: idx + 1}, true
}

// Rank returns the 1-based rank of the player with the given ID.
func (l *Leaderboard) Rank(id string) (int, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	score, ok := l.scores[id]
	if !ok {
		return 0, false
	}
	return l.searchLocked(entry{id: id, score: score}) + 1, true
}

// Top returns the first n players in ranking order. If n <= 0 it returns
// nil; if n exceeds the player count it returns all players.
func (l *Leaderboard) Top(n int) []Player {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if n <= 0 {
		return nil
	}
	if n > len(l.sorted) {
		n = len(l.sorted)
	}
	return copyPlayers(l.sorted[:n], 1)
}

// Around returns up to n players ranked immediately before the given player,
// the player itself, and up to n players ranked immediately after, in ranking
// order. The second result is false if the player does not exist.
func (l *Leaderboard) Around(id string, n int) ([]Player, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	score, ok := l.scores[id]
	if !ok {
		return nil, false
	}
	if n < 0 {
		n = 0
	}
	idx := l.searchLocked(entry{id: id, score: score})
	lo := idx - n
	if lo < 0 {
		lo = 0
	}
	hi := idx + n + 1
	if hi > len(l.sorted) {
		hi = len(l.sorted)
	}
	return copyPlayers(l.sorted[lo:hi], lo+1), true
}

// Snapshot returns all players in ranking order.
func (l *Leaderboard) Snapshot() []Player {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return copyPlayers(l.sorted, 1)
}

// copyPlayers copies entries into fresh Player values with ranks starting at
// firstRank, so callers can never mutate internal state.
func copyPlayers(entries []entry, firstRank int) []Player {
	players := make([]Player, len(entries))
	for i, e := range entries {
		players[i] = Player{ID: e.id, Score: e.score, Rank: firstRank + i}
	}
	return players
}

// lessLocked reports whether a ranks before b: higher score first, ties
// broken by ascending ID.
func lessLocked(a, b entry) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	return a.id < b.id
}

// searchLocked returns the index of e in the sorted slice. Callers only use
// it for entries that are present.
func (l *Leaderboard) searchLocked(e entry) int {
	return sort.Search(len(l.sorted), func(i int) bool {
		return !lessLocked(l.sorted[i], e)
	})
}

// insertLocked inserts e into its sorted position.
func (l *Leaderboard) insertLocked(e entry) {
	idx := sort.Search(len(l.sorted), func(i int) bool {
		return !lessLocked(l.sorted[i], e)
	})
	l.sorted = append(l.sorted, entry{})
	copy(l.sorted[idx+1:], l.sorted[idx:])
	l.sorted[idx] = e
}

// removeLocked removes the entry matching (id, score).
func (l *Leaderboard) removeLocked(id string, score int64) {
	idx := l.searchLocked(entry{id: id, score: score})
	l.sorted = append(l.sorted[:idx], l.sorted[idx+1:]...)
}
