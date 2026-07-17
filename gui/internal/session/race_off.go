//go:build !race

package session

// raceEnabled reports whether the binary was built with the Go race
// detector (`go test -race`). See raceSkipReason for why the round-trip
// tests consult it.
const raceEnabled = false
