package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

const flushInterval = 3 * time.Second

type Store struct {
	mutex     sync.Mutex
	path      string
	data      map[string]map[int64]int64
	dirty     bool
	lastFlush time.Time
}

var Global = New("cassfr-progress.json")

func New(path string) *Store {
	return &Store{path: path, data: make(map[string]map[int64]int64)}
}

func (s *Store) SetPath(path string) {
	s.mutex.Lock()
	s.path = path
	s.mutex.Unlock()
}

func Key(assetType string, isGroup bool, creatorID int64) string {
	kind := "u"
	if isGroup {
		kind = "g"
	}
	return fmt.Sprintf("%s:%s:%d", assetType, kind, creatorID)
}

func (s *Store) Load() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var parsed map[string]map[string]int64
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}

	s.data = make(map[string]map[int64]int64, len(parsed))
	for key, entries := range parsed {
		group := make(map[int64]int64, len(entries))
		for oldText, newID := range entries {
			oldID, err := strconv.ParseInt(oldText, 10, 64)
			if err != nil {
				continue
			}
			group[oldID] = newID
		}
		s.data[key] = group
	}
	return nil
}

func (s *Store) Lookup(key string, oldID int64) (int64, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newID, ok := s.data[key][oldID]
	return newID, ok
}

func (s *Store) Add(key string, oldID, newID int64) {
	s.mutex.Lock()
	group := s.data[key]
	if group == nil {
		group = make(map[int64]int64)
		s.data[key] = group
	}
	group[oldID] = newID
	s.dirty = true
	due := time.Since(s.lastFlush) >= flushInterval
	s.mutex.Unlock()

	if due {
		s.Flush()
	}
}

func (s *Store) Count() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	total := 0
	for _, group := range s.data {
		total += len(group)
	}
	return total
}

func (s *Store) Flush() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.dirty {
		return nil
	}

	out := make(map[string]map[string]int64, len(s.data))
	for key, group := range s.data {
		entries := make(map[string]int64, len(group))
		for oldID, newID := range group {
			entries[strconv.FormatInt(oldID, 10)] = newID
		}
		out[key] = entries
	}

	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}

	s.dirty = false
	s.lastFlush = time.Now()
	return nil
}

func (s *Store) Clear() int {
	s.mutex.Lock()
	removed := 0
	for _, group := range s.data {
		removed += len(group)
	}
	s.data = make(map[string]map[int64]int64)
	s.dirty = true
	s.mutex.Unlock()

	s.Flush()
	return removed
}
