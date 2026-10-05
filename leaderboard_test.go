package leaderboard

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func mustNew(t *testing.T, capacity int) *Leaderboard {
	t.Helper()
	lb, err := New(capacity)
	if err != nil {
		t.Fatalf("New(%d): %v", capacity, err)
	}
	return lb
}

func mustSet(t *testing.T, lb *Leaderboard, id string, score int64) {
	t.Helper()
	if err := lb.SetScore(id, score); err != nil {
		t.Fatalf("SetScore(%q, %d): %v", id, score, err)
	}
}

func ids(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

func TestSetScoreCreatesAndGets(t *testing.T) {
	lb := mustNew(t, 3)
	mustSet(t, lb, "alice", 42)

	score, ok := lb.Get("alice")
	if !ok || score != 42 {
		t.Fatalf("Get(alice) = %d, %v; want 42, true", score, ok)
	}
	if _, ok := lb.Get("ghost"); ok {
		t.Fatal("Get(ghost) should report missing")
	}
	if lb.Len() != 1 || lb.Cap() != 3 {
		t.Fatalf("Len/Cap = %d/%d; want 1/3", lb.Len(), lb.Cap())
	}
}

func TestSetScoreOverwrites(t *testing.T) {
	lb := mustNew(t, 2)
	mustSet(t, lb, "alice", 10)
	mustSet(t, lb, "alice", 99)

	if score, _ := lb.Get("alice"); score != 99 {
		t.Fatalf("score = %d; want 99", score)
	}
	if lb.Len() != 1 {
		t.Fatalf("Len = %d; want 1 (overwrite must not duplicate)", lb.Len())
	}
}

func TestAddScoreAccumulatesAndCreates(t *testing.T) {
	lb := mustNew(t, 2)
	if err := lb.AddScore("alice", 5); err != nil {
		t.Fatalf("AddScore create: %v", err)
	}
	if err := lb.AddScore("alice", 7); err != nil {
		t.Fatalf("AddScore accumulate: %v", err)
	}
	if err := lb.AddScore("alice", -3); err != nil {
		t.Fatalf("AddScore negative delta: %v", err)
	}
	if score, _ := lb.Get("alice"); score != 9 {
		t.Fatalf("score = %d; want 9", score)
	}
}

func TestRemove(t *testing.T) {
	lb := mustNew(t, 2)
	mustSet(t, lb, "alice", 1)

	if !lb.Remove("alice") {
		t.Fatal("Remove(alice) = false; want true")
	}
	if lb.Remove("alice") {
		t.Fatal("Remove(alice) again = true; want false")
	}
	if lb.Remove("ghost") {
		t.Fatal("Remove(ghost) = true; want false")
	}
	if lb.Len() != 0 {
		t.Fatalf("Len = %d; want 0", lb.Len())
	}
	// Slot must be reusable after removal.
	mustSet(t, lb, "bob", 2)
}

func TestCapacityRejectsNewButAllowsUpdates(t *testing.T) {
	lb := mustNew(t, 2)
	mustSet(t, lb, "a", 1)
	mustSet(t, lb, "b", 2)

	if err := lb.SetScore("c", 3); !errors.Is(err, ErrFull) {
		t.Fatalf("SetScore new at capacity: %v; want ErrFull", err)
	}
	if err := lb.AddScore("c", 3); !errors.Is(err, ErrFull) {
		t.Fatalf("AddScore new at capacity: %v; want ErrFull", err)
	}
	// Existing players keep updating fine.
	mustSet(t, lb, "a", 100)
	if err := lb.AddScore("b", 100); err != nil {
		t.Fatalf("AddScore existing at capacity: %v", err)
	}
	if lb.Len() != 2 {
		t.Fatalf("Len = %d; want 2", lb.Len())
	}
	if got := ids(lb.Snapshot()); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Fatalf("order = %v; want [b a]", got)
	}
}

func TestTieBreaksByIDAscending(t *testing.T) {
	lb := mustNew(t, 5)
	for _, id := range []string{"delta", "alpha", "charlie", "bravo"} {
		mustSet(t, lb, id, 50)
	}
	mustSet(t, lb, "top", 51)

	want := []string{"top", "alpha", "bravo", "charlie", "delta"}
	if got := ids(lb.Snapshot()); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v; want %v", got, want)
	}
}

func TestRank(t *testing.T) {
	lb := mustNew(t, 4)
	mustSet(t, lb, "a", 10)
	mustSet(t, lb, "b", 30)
	mustSet(t, lb, "c", 20)

	for id, want := range map[string]int{"b": 1, "c": 2, "a": 3} {
		if rank, ok := lb.Rank(id); !ok || rank != want {
			t.Fatalf("Rank(%q) = %d, %v; want %d, true", id, rank, ok, want)
		}
	}
	if _, ok := lb.Rank("ghost"); ok {
		t.Fatal("Rank(ghost) should report missing")
	}
	// Rank tracks score changes.
	mustSet(t, lb, "a", 40)
	if rank, _ := lb.Rank("a"); rank != 1 {
		t.Fatalf("Rank(a) after update = %d; want 1", rank)
	}
}

func TestTop(t *testing.T) {
	lb := mustNew(t, 5)
	mustSet(t, lb, "a", 10)
	mustSet(t, lb, "b", 30)
	mustSet(t, lb, "c", 20)

	if got := ids(lb.Top(2)); !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatalf("Top(2) = %v; want [b c]", got)
	}
	if got := lb.Top(99); len(got) != 3 {
		t.Fatalf("Top(99) len = %d; want 3 (clamped to size)", len(got))
	}
	if got := lb.Top(0); len(got) != 0 {
		t.Fatalf("Top(0) len = %d; want 0", len(got))
	}
}

func TestAround(t *testing.T) {
	lb := mustNew(t, 6)
	for i, id := range []string{"a", "b", "c", "d", "e"} {
		mustSet(t, lb, id, int64(50-i*10)) // a=50 b=40 c=30 d=20 e=10
	}

	around, ok := lb.Around("c", 1, 1)
	if !ok {
		t.Fatal("Around(c) should exist")
	}
	if got := ids(around); !reflect.DeepEqual(got, []string{"b", "c", "d"}) {
		t.Fatalf("Around(c,1,1) = %v; want [b c d]", got)
	}

	// Clamped at the edges.
	around, _ = lb.Around("a", 2, 1)
	if got := ids(around); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("Around(a,2,1) = %v; want [a b]", got)
	}
	around, _ = lb.Around("e", 2, 5)
	if got := ids(around); !reflect.DeepEqual(got, []string{"c", "d", "e"}) {
		t.Fatalf("Around(e,2,5) = %v; want [c d e]", got)
	}
	// Self only.
	around, _ = lb.Around("c", 0, 0)
	if got := ids(around); !reflect.DeepEqual(got, []string{"c"}) {
		t.Fatalf("Around(c,0,0) = %v; want [c]", got)
	}
	if _, ok := lb.Around("ghost", 1, 1); ok {
		t.Fatal("Around(ghost) should report missing")
	}
}

func TestSnapshotStableOrder(t *testing.T) {
	lb := mustNew(t, 8)
	// Insert in a scrambled order; output must always be rank order.
	for _, e := range []Entry{
		{"m", 5}, {"a", 9}, {"z", 1}, {"b", 9}, {"k", 5}, {"q", 7},
	} {
		mustSet(t, lb, e.ID, e.Score)
	}
	want := []Entry{
		{"a", 9}, {"b", 9}, {"q", 7}, {"k", 5}, {"m", 5}, {"z", 1},
	}
	for i := 0; i < 3; i++ {
		if got := lb.Snapshot(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Snapshot run %d = %v; want %v", i, got, want)
		}
	}
}

func TestResultsAreIndependentCopies(t *testing.T) {
	lb := mustNew(t, 3)
	mustSet(t, lb, "a", 3)
	mustSet(t, lb, "b", 2)
	mustSet(t, lb, "c", 1)

	mutate := func(entries []Entry) {
		for i := range entries {
			entries[i].ID = "hacked"
			entries[i].Score = -999
		}
	}
	mutate(lb.Top(2))
	mutate(lb.Snapshot())
	if around, ok := lb.Around("b", 1, 1); ok {
		mutate(around)
	} else {
		t.Fatal("Around(b) should exist")
	}

	want := []Entry{{"a", 3}, {"b", 2}, {"c", 1}}
	if got := lb.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("internal state corrupted by caller: got %v; want %v", got, want)
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		if _, err := New(capacity); !errors.Is(err, ErrInvalidCapacity) {
			t.Fatalf("New(%d) err = %v; want ErrInvalidCapacity", capacity, err)
		}
	}
	lb := mustNew(t, 1)
	if err := lb.SetScore("", 1); !errors.Is(err, ErrEmptyID) {
		t.Fatalf("SetScore empty ID err = %v; want ErrEmptyID", err)
	}
	if err := lb.AddScore("", 1); !errors.Is(err, ErrEmptyID) {
		t.Fatalf("AddScore empty ID err = %v; want ErrEmptyID", err)
	}
	if lb.Remove("") {
		t.Fatal("Remove empty ID = true; want false")
	}
	if got := lb.Top(-5); len(got) != 0 {
		t.Fatalf("Top(-5) len = %d; want 0", len(got))
	}
	if lb.Len() != 0 {
		t.Fatalf("Len = %d; want 0 after rejected calls", lb.Len())
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	lb := mustNew(t, 64)
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			id := fmt.Sprintf("player-%d", worker%16)
			for i := 0; i < 500; i++ {
				_ = lb.SetScore(id, int64(i))
				_ = lb.AddScore(id, 1)
				_, _ = lb.Get(id)
				_, _ = lb.Rank(id)
				_ = lb.Top(5)
				_, _ = lb.Around(id, 2, 2)
				_ = lb.Snapshot()
				if i%3 == 0 {
					_ = lb.Remove(fmt.Sprintf("player-%d", (worker+1)%16))
				}
			}
		}(worker)
	}
	wg.Wait()
	if lb.Len() > lb.Cap() {
		t.Fatalf("Len = %d exceeds Cap = %d", lb.Len(), lb.Cap())
	}
}

func TestConcurrentCapacityNeverExceeded(t *testing.T) {
	const capacity = 32
	lb := mustNew(t, capacity)
	var wg sync.WaitGroup
	// 256 goroutines race to claim 32 slots with unique IDs.
	for worker := 0; worker < 256; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			_ = lb.AddScore(fmt.Sprintf("p%d", worker), 1)
		}(worker)
	}
	wg.Wait()
	if got := lb.Len(); got != capacity {
		t.Fatalf("Len = %d; want exactly %d (all slots filled, none over)", got, capacity)
	}
}
