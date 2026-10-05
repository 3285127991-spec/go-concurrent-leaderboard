// Command demo exercises the leaderboard library end to end.
package main

import (
	"fmt"

	"leaderboard"
)

func printEntries(title string, startRank int, entries []leaderboard.Entry) {
	fmt.Println(title)
	for i, e := range entries {
		fmt.Printf("  %2d. %-8s %d\n", startRank+i, e.ID, e.Score)
	}
}

func main() {
	lb, err := leaderboard.New(5)
	if err != nil {
		panic(err)
	}
	fmt.Printf("created leaderboard: capacity=%d players=%d\n\n", lb.Cap(), lb.Len())

	// SetScore: absolute scores.
	for _, e := range []leaderboard.Entry{
		{ID: "alice", Score: 100},
		{ID: "bob", Score: 80},
		{ID: "carol", Score: 100},
		{ID: "dave", Score: 60},
		{ID: "erin", Score: 90},
	} {
		if err := lb.SetScore(e.ID, e.Score); err != nil {
			fmt.Println("SetScore error:", err)
		}
	}
	printEntries("after SetScore (note alice/carol tie broken by ID):", 1, lb.Snapshot())

	// AddScore: accumulate, and update an existing player at full capacity.
	must := func(err error) {
		if err != nil {
			fmt.Println("error:", err)
		}
	}
	must(lb.AddScore("bob", 30))   // 80 -> 110, takes the lead
	must(lb.AddScore("dave", -15)) // 60 -> 45
	fmt.Println()
	printEntries("after AddScore bob+30, dave-15:", 1, lb.Snapshot())

	// Capacity: board is full, so a brand-new player is rejected...
	if err := lb.SetScore("frank", 999); err != nil {
		fmt.Println("\nSetScore frank at full capacity:", err)
	}
	// ...but removing one frees a slot.
	fmt.Println("Remove dave:", lb.Remove("dave"))
	fmt.Println("Remove dave again:", lb.Remove("dave"))
	must(lb.SetScore("frank", 70))

	// Queries.
	if rank, ok := lb.Rank("carol"); ok {
		fmt.Printf("\ncarol rank: %d\n", rank)
	}
	if score, ok := lb.Get("bob"); ok {
		fmt.Printf("bob score: %d\n", score)
	}
	fmt.Println()
	printEntries("Top 3:", 1, lb.Top(3))
	fmt.Println()
	if around, ok := lb.Around("carol", 1, 1); ok {
		rank, _ := lb.Rank("carol")
		printEntries("Around carol (1 before, 1 after):", rank-1, around)
	}
	fmt.Printf("\nfinal state: players=%d capacity=%d\n", lb.Len(), lb.Cap())
}
