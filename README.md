# go-concurrent-leaderboard

A fixed-capacity, concurrency-safe, in-memory leaderboard written in pure Go.
Library + CLI demo + tests only: no network service, no persistence, no GUI,
no database, no third-party dependencies.

## Features

- Fixed maximum player count, set at creation; never exceeded, even under
  concurrent writes.
- Players are identified by a unique non-empty string ID with an int64 score.
- Ranking: score descending; ties broken by player ID ascending
  (lexicographic). Ranks are 1-based.
- Safe for concurrent use by multiple goroutines (guarded by
  `sync.RWMutex`; verified with the race detector).
- All query results are independent copies: mutating a returned slice or
  element never affects internal state.

## API

```go
lb, err := leaderboard.New(1000) // capacity = 1000 players
```

| Method | Description |
| --- | --- |
| `SetScore(id, score) error` | Set an absolute score; creates the player if missing. |
| `AddScore(id, delta) error` | Add a delta to the score; creates the player if missing. |
| `Remove(id) bool` | Delete a player; `false` if the player did not exist. |
| `Get(id) (Player, bool)` | Fetch one player with current score and rank. |
| `Rank(id) (int, bool)` | 1-based rank of a player. |
| `Top(n) []Player` | First n players in ranking order (`nil` for `n <= 0`). |
| `Around(id, n) ([]Player, bool)` | Up to n players before and after `id`, including the player. |
| `Snapshot() []Player` | All players in ranking order. |
| `Len() int` / `Capacity() int` | Current player count / maximum capacity. |

Errors:

- `ErrInvalidCapacity` — `New` with capacity <= 0.
- `ErrEmptyID` — empty player ID passed to `SetScore`/`AddScore`.
- `ErrFull` — adding a **new** player would exceed capacity. Updating an
  existing player always succeeds, even when the leaderboard is full.

`Player` is a snapshot value: `{ ID string; Score int64; Rank int }`.

## CLI demo

```sh
go run ./cmd/demo
```

Demonstrates SetScore/AddScore, capacity rejection, tie-breaking, Top,
Around, Rank, Remove, Snapshot, and concurrent updates from goroutines.

## Tests

Run the full suite (requires only the Go toolchain):

```sh
go test ./... -count=1
```

Run with the race detector (on Windows this needs CGO and a gcc toolchain,
e.g. MSYS2 `gcc` on `PATH`):

```sh
go test -race ./... -count=1
```

Coverage includes: player creation, score overwrite, score accumulation,
removal (including missing players), capacity boundaries, tie-break ordering,
Rank, Top, Around, Snapshot ordering, result-copy isolation, invalid
arguments, concurrent reads/writes, and the guarantee that the player count
never exceeds capacity.

## Layout

- `leaderboard.go` — the library (package `leaderboard`).
- `leaderboard_test.go` — automated tests.
- `cmd/demo/main.go` — CLI demonstration.
