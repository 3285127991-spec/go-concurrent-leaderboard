package leaderboard_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"leaderboard"
)

func mustNew(t *testing.T, capacity int) *leaderboard.Leaderboard {
	t.Helper()
	lb, err := leaderboard.New(capacity)
	if err != nil {
		t.Fatalf("New(%d): unexpected error: %v", capacity, err)
	}
	return lb
}

func mustSet(t *testing.T, lb *leaderboard.Leaderboard, id string, score int64) {
	t.Helper()
	if err := lb.SetScore(id, score); err != nil {
		t.Fatalf("SetScore(%q, %d): unexpected error: %v", id, score, err)
	}
}

func mustAdd(t *testing.T, lb *leaderboard.Leaderboard, id string, delta int64) {
	t.Helper()
	if err := lb.AddScore(id, delta); err != nil {
		t.Fatalf("AddScore(%q, %d): unexpected error: %v", id, delta, err)
	}
}

func wantOrder(t *testing.T, players []leaderboard.Player, firstRank int, wantIDs ...string) {
	t.Helper()
	if len(players) != len(wantIDs) {
		t.Fatalf("got %d players, want %d: %v", len(players), len(wantIDs), players)
	}
	for i, id := range wantIDs {
		if players[i].ID != id {
			t.Fatalf("position %d: got %q, want %q (full: %v)", i, players[i].ID, id, players)
		}
		if players[i].Rank != firstRank+i {
			t.Fatalf("position %d: got rank %d, want %d", i, players[i].Rank, firstRank+i)
		}
	}
}

func TestNewInvalidCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1, -100} {
		if _, err := leaderboard.New(capacity); !errors.Is(err, leaderboard.ErrInvalidCapacity) {
			t.Errorf("New(%d): got %v, want ErrInvalidCapacity", capacity, err)
		}
	}
}

func TestSetScoreCreateAndOverwrite(t *testing.T) {
	lb := mustNew(t, 10)
	mustSet(t, lb, "alice", 100)
	mustSet(t, lb, "bob", 50)

	p, ok := lb.Get("alice")
	if !ok || p.Score != 100 || p.Rank != 1 {
		t.Fatalf("Get(alice) = %+v, %v; want score 100 rank 1", p, ok)
	}

	// Overwrite: bob jumps ahead of alice.
	mustSet(t, lb, "bob", 150)
	p, ok = lb.Get("bob")
	if !ok || p.Score != 150 || p.Rank != 1 {
		t.Fatalf("Get(bob) = %+v, %v; want score 150 rank 1", p, ok)
	}
	if p, _ := lb.Get("alice"); p.Rank != 2 {
		t.Fatalf("alice rank = %d, want 2", p.Rank)
	}
	if lb.Len() != 2 {
		t.Fatalf("Len() = %d, want 2 (overwrite must not duplicate)", lb.Len())
	}
}

func TestAddScoreAccumulateAndCreate(t *testing.T) {
	lb := mustNew(t, 10)
	mustAdd(t, lb, "alice", 30)  // creates alice with 30
	mustAdd(t, lb, "alice", 25)  // 55
	mustAdd(t, lb, "alice", -10) // 45

	p, ok := lb.Get("alice")
	if !ok || p.Score != 45 {
		t.Fatalf("Get(alice) = %+v, %v; want score 45", p, ok)
	}
}

func TestEmptyIDRejected(t *testing.T) {
	lb := mustNew(t, 10)
	if err := lb.SetScore("", 1); !errors.Is(err, leaderboard.ErrEmptyID) {
		t.Errorf("SetScore empty ID: got %v, want ErrEmptyID", err)
	}
	if err := lb.AddScore("", 1); !errors.Is(err, leaderboard.ErrEmptyID) {
		t.Errorf("AddScore empty ID: got %v, want ErrEmptyID", err)
	}
	if lb.Len() != 0 {
		t.Errorf("Len() = %d, want 0", lb.Len())
	}
}

func TestRemove(t *testing.T) {
	lb := mustNew(t, 10)
	mustSet(t, lb, "alice", 100)
	mustSet(t, lb, "bob", 50)

	if !lb.Remove("alice") {
		t.Fatal("Remove(alice) = false, want true")
	}
	if _, ok := lb.Get("alice"); ok {
		t.Fatal("Get(alice) still present after Remove")
	}
	if _, ok := lb.Rank("alice"); ok {
		t.Fatal("Rank(alice) still present after Remove")
	}
	if lb.Remove("alice") {
		t.Fatal("Remove(alice) again = true, want false")
	}
	if lb.Remove("ghost") {
		t.Fatal("Remove(ghost) = true, want false for missing player")
	}
	if lb.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", lb.Len())
	}
	// Remaining player keeps correct rank.
	if rank, ok := lb.Rank("bob"); !ok || rank != 1 {
		t.Fatalf("Rank(bob) = %d, %v; want 1, true", rank, ok)
	}
}

func TestCapacityBoundary(t *testing.T) {
	lb := mustNew(t, 2)
	mustSet(t, lb, "a", 10)
	mustAdd(t, lb, "b", 20)

	if err := lb.SetScore("c", 30); !errors.Is(err, leaderboard.ErrFull) {
		t.Fatalf("SetScore new player when full: got %v, want ErrFull", err)
	}
	if err := lb.AddScore("c", 30); !errors.Is(err, leaderboard.ErrFull) {
		t.Fatalf("AddScore new player when full: got %v, want ErrFull", err)
	}
	// Existing players can still be updated while full.
	mustSet(t, lb, "a", 100)
	mustAdd(t, lb, "b", 100)
	if p, _ := lb.Get("a"); p.Score != 100 {
		t.Fatalf("a score = %d, want 100", p.Score)
	}
	if p, _ := lb.Get("b"); p.Score != 120 {
		t.Fatalf("b score = %d, want 120", p.Score)
	}
	if lb.Len() != 2 || lb.Capacity() != 2 {
		t.Fatalf("Len=%d Capacity=%d, want 2/2", lb.Len(), lb.Capacity())
	}
	// After a removal there is room again.
	if !lb.Remove("a") {
		t.Fatal("Remove(a) = false, want true")
	}
	mustSet(t, lb, "c", 30)
	if lb.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", lb.Len())
	}
}

func TestTieBreakByIDAscending(t *testing.T) {
	lb := mustNew(t, 10)
	mustSet(t, lb, "charlie", 100)
	mustSet(t, lb, "alice", 100)
	mustSet(t, lb, "bob", 100)
	mustSet(t, lb, "zoe", 200)
	mustSet(t, lb, "ann", 50)

	wantOrder(t, lb.Snapshot(), 1, "zoe", "alice", "bob", "charlie", "ann")
}

func TestRank(t *testing.T) {
	lb := mustNew(t, 10)
	mustSet(t, lb, "a", 10)
	mustSet(t, lb, "b", 30)
	mustSet(t, lb, "c", 20)

	for id, want := range map[string]int{"b": 1, "c": 2, "a": 3} {
		if rank, ok := lb.Rank(id); !ok || rank != want {
			t.Errorf("Rank(%q) = %d, %v; want %d, true", id, rank, ok, want)
		}
	}
	if _, ok := lb.Rank("missing"); ok {
		t.Error("Rank(missing) ok = true, want false")
	}
}

func TestTop(t *testing.T) {
	lb := mustNew(t, 10)
	mustSet(t, lb, "a", 10)
	mustSet(t, lb, "b", 30)
	mustSet(t, lb, "c", 20)

	wantOrder(t, lb.Top(2), 1, "b", "c")
	wantOrder(t, lb.Top(100), 1, "b", "c", "a") // larger than player count
	if got := lb.Top(0); got != nil {
		t.Errorf("Top(0) = %v, want nil", got)
	}
	if got := lb.Top(-5); got != nil {
		t.Errorf("Top(-5) = %v, want nil", got)
	}
}

func TestAround(t *testing.T) {
	lb := mustNew(t, 10)
	// ranks: e(50) d(40) c(30) b(20) a(10)
	mustSet(t, lb, "a", 10)
	mustSet(t, lb, "b", 20)
	mustSet(t, lb, "c", 30)
	mustSet(t, lb, "d", 40)
	mustSet(t, lb, "e", 50)

	around, ok := lb.Around("c", 1)
	if !ok {
		t.Fatal("Around(c, 1) ok = false, want true")
	}
	wantOrder(t, around, 2, "d", "c", "b")

	// Player at the top has nobody before them.
	around, _ = lb.Around("e", 2)
	wantOrder(t, around, 1, "e", "d", "c")

	// Player at the bottom has nobody after them.
	around, _ = lb.Around("a", 2)
	wantOrder(t, around, 3, "c", "b", "a")

	// n = 0 returns only the player itself.
	around, _ = lb.Around("c", 0)
	wantOrder(t, around, 3, "c")

	// Large n clamps to the whole board.
	around, _ = lb.Around("c", 100)
	wantOrder(t, around, 1, "e", "d", "c", "b", "a")

	if _, ok := lb.Around("missing", 1); ok {
		t.Error("Around(missing, 1) ok = true, want false")
	}
}

func TestSnapshotStableOrder(t *testing.T) {
	lb := mustNew(t, 10)
	mustSet(t, lb, "b", 20)
	mustSet(t, lb, "a", 20)
	mustSet(t, lb, "c", 30)

	first := lb.Snapshot()
	wantOrder(t, first, 1, "c", "a", "b")
	// Repeated snapshots return the same order.
	wantOrder(t, lb.Snapshot(), 1, "c", "a", "b")
	// Scores are carried along.
	if first[0].Score != 30 || first[1].Score != 20 || first[2].Score != 20 {
		t.Errorf("snapshot scores = %d,%d,%d; want 30,20,20",
			first[0].Score, first[1].Score, first[2].Score)
	}
}

func TestResultCopyIsolation(t *testing.T) {
	lb := mustNew(t, 10)
	mustSet(t, lb, "a", 10)
	mustSet(t, lb, "b", 20)
	mustSet(t, lb, "c", 30)

	top := lb.Top(3)
	snap := lb.Snapshot()
	around, _ := lb.Around("b", 1)

	// Mutate every returned value as hard as possible.
	top[0].ID = "hacked"
	top[0].Score = -999
	top[0].Rank = 999
	snap[1].Score = 1 << 40
	around[0].ID = "hacked"

	// Internal state must be unaffected.
	wantOrder(t, lb.Snapshot(), 1, "c", "b", "a")
	if p, _ := lb.Get("c"); p.Score != 30 || p.Rank != 1 {
		t.Errorf("Get(c) = %+v, want score 30 rank 1", p)
	}
	if p, _ := lb.Get("b"); p.Score != 20 || p.Rank != 2 {
		t.Errorf("Get(b) = %+v, want score 20 rank 2", p)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	lb := mustNew(t, 64)
	const workers = 8
	const iterations = 500

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			id := fmt.Sprintf("player-%d", w%4)
			for i := 0; i < iterations; i++ {
				switch i % 6 {
				case 0:
					_ = lb.SetScore(id, int64(i))
				case 1:
					_ = lb.AddScore(id, 1)
				case 2:
					lb.Remove(id)
				case 3:
					lb.Top(3)
				case 4:
					lb.Rank(id)
					lb.Get(id)
					lb.Around(id, 2)
				case 5:
					lb.Snapshot()
					lb.Len()
				}
			}
		}(w)
	}
	wg.Wait()

	if lb.Len() > lb.Capacity() {
		t.Fatalf("Len() = %d exceeds capacity %d", lb.Len(), lb.Capacity())
	}
}

func TestConcurrentCapacityNeverExceeded(t *testing.T) {
	const capacity = 16
	lb := mustNew(t, capacity)
	const goroutines = 64

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			id := fmt.Sprintf("contender-%d", g)
			for i := 0; i < 100; i++ {
				err := lb.SetScore(id, int64(i))
				if err != nil && !errors.Is(err, leaderboard.ErrFull) {
					t.Errorf("SetScore: unexpected error: %v", err)
					return
				}
				if lb.Len() > capacity {
					t.Errorf("Len() = %d exceeds capacity %d", lb.Len(), capacity)
					return
				}
				if i%3 == 0 {
					lb.Remove(id)
				}
			}
		}(g)
	}
	wg.Wait()

	if lb.Len() > capacity {
		t.Fatalf("final Len() = %d exceeds capacity %d", lb.Len(), capacity)
	}
	// Snapshot must be consistent with Len.
	if got := len(lb.Snapshot()); got != lb.Len() {
		t.Fatalf("Snapshot length %d != Len() %d", got, lb.Len())
	}
}
