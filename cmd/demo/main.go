// Command demo is a small CLI walkthrough of the leaderboard package.
package main

import (
	"errors"
	"fmt"
	"sync"

	"leaderboard"
)

func printPlayers(title string, players []leaderboard.Player) {
	fmt.Println(title)
	for _, p := range players {
		fmt.Printf("  #%d  %-10s %d\n", p.Rank, p.ID, p.Score)
	}
}

func main() {
	lb, err := leaderboard.New(5)
	if err != nil {
		panic(err)
	}
	fmt.Printf("created leaderboard: capacity=%d players=%d\n\n", lb.Capacity(), lb.Len())

	// Set absolute scores and accumulate deltas.
	must(lb.SetScore("alice", 100))
	must(lb.SetScore("bob", 80))
	must(lb.SetScore("carol", 100))
	must(lb.AddScore("dave", 60))
	must(lb.AddScore("bob", 30))  // bob: 80 + 30 = 110
	must(lb.AddScore("bob", -10)) // bob: 110 - 10 = 100

	// Capacity is 5: one more player fits, the next is rejected.
	must(lb.SetScore("erin", 40))
	if err := lb.SetScore("frank", 10); errors.Is(err, leaderboard.ErrFull) {
		fmt.Println("frank rejected: leaderboard is full")
	}
	// Existing players can still be updated when full.
	must(lb.AddScore("erin", 5))

	printPlayers("\nTop 3:", lb.Top(3))

	if around, ok := lb.Around("carol", 1); ok {
		printPlayers("\nAround carol (1 before/after):", around)
	}

	if rank, ok := lb.Rank("alice"); ok {
		fmt.Printf("\nalice rank: %d\n", rank)
	}

	if lb.Remove("dave") {
		fmt.Println("removed dave")
	}
	if !lb.Remove("nobody") {
		fmt.Println("remove nobody: player not found")
	}

	// Concurrent updates and queries from multiple goroutines.
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = lb.AddScore(id, 1)
				_ = lb.Top(2)
				_ = lb.Snapshot()
			}
		}(fmt.Sprintf("player-%c", 'a'+rune(worker)))
	}
	wg.Wait()

	printPlayers("\nFinal snapshot:", lb.Snapshot())
	fmt.Printf("\nplayers=%d capacity=%d\n", lb.Len(), lb.Capacity())
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
