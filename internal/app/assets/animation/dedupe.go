package animation

import "sync"

type dedupeKey struct {
	sum   [32]byte
	mode  string
	group int64
}

type dedupeEntry struct {
	done chan struct{}
	id   int64
}

var dedupeMu sync.Mutex
var dedupeMap = map[dedupeKey]*dedupeEntry{}

func dedupeClaim(k dedupeKey) (*dedupeEntry, bool) {
	dedupeMu.Lock()
	defer dedupeMu.Unlock()
	if e, ok := dedupeMap[k]; ok {
		return e, false
	}
	e := &dedupeEntry{done: make(chan struct{})}
	dedupeMap[k] = e
	return e, true
}

func dedupeFinish(k dedupeKey, e *dedupeEntry, id int64) {
	dedupeMu.Lock()
	e.id = id
	if id == 0 {
		delete(dedupeMap, k)
	}
	dedupeMu.Unlock()
	close(e.done)
}
