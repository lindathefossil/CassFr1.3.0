// Package convert hands animation data to the Studio plugin so it can be converted
// between R15 and R6 (only the engine can read/write KeyframeSequences reliably) and
// waits for the converted data to come back.
//
// Flow: the uploader calls Global.Convert (blocks) -> the plugin polls GET /convert/next
// -> converts -> POSTs the result to /convert/result -> Convert returns.
package convert

import (
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	ModeR15ToR6 = "r15_to_r6"
	ModeR6ToR15 = "r6_to_r15"

	// How long the plugin may hold a job before we give up on it.
	inflightTimeout = 120 * time.Second
	// How long the plugin may stay silent before queued jobs are failed.
	pluginSilence = 30 * time.Second
)

func ValidMode(mode string) bool {
	return mode == ModeR15ToR6 || mode == ModeR6ToR15
}

func Label(mode string) string {
	switch mode {
	case ModeR15ToR6:
		return "R15 -> R6"
	case ModeR6ToR15:
		return "R6 -> R15"
	}
	return mode
}

// Job is what the plugin receives. Data is base64.
type Job struct {
	ID   int64  `json:"id"`
	Mode string `json:"mode"`
	Data string `json:"data"`
}

// Result is what the plugin sends back. Data is base64.
type Result struct {
	ID      int64  `json:"id"`
	Data    string `json:"data"`
	Skipped bool   `json:"skipped"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

// Outcome of a conversion. When Skipped is true Data is the original, untouched data.
type Outcome struct {
	Data    []byte
	Skipped bool
	Message string
}

type entry struct {
	Job
	enqueued time.Time
	sentAt   time.Time
	done     chan *Result
}

type Broker struct {
	mutex    sync.Mutex
	nextID   int64
	pending  []*entry
	inflight map[int64]*entry
	lastPoll time.Time
}

var Global = New()

func New() *Broker {
	return &Broker{inflight: make(map[int64]*entry)}
}

// Convert queues data for the plugin and blocks until it answers or something goes wrong.
func (b *Broker) Convert(mode string, data []byte) (*Outcome, error) {
	e := &entry{
		enqueued: time.Now(),
		done:     make(chan *Result, 1),
	}

	b.mutex.Lock()
	b.nextID++
	e.Job = Job{ID: b.nextID, Mode: mode, Data: base64.StdEncoding.EncodeToString(data)}
	b.pending = append(b.pending, e)
	b.mutex.Unlock()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case res := <-e.done:
			if res.Error != "" {
				return nil, errors.New(res.Error)
			}
			if res.Skipped {
				return &Outcome{Data: data, Skipped: true, Message: res.Message}, nil
			}

			out, err := base64.StdEncoding.DecodeString(res.Data)
			if err != nil {
				return nil, fmt.Errorf("plugin sent invalid data: %w", err)
			}
			if len(out) == 0 {
				return nil, errors.New("plugin sent an empty animation")
			}
			return &Outcome{Data: out}, nil
		case <-ticker.C:
			if err := b.check(e); err != nil {
				b.forget(e.ID)
				return nil, err
			}
		}
	}
}

func (b *Broker) check(e *entry) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	now := time.Now()
	if !e.sentAt.IsZero() {
		if now.Sub(e.sentAt) > inflightTimeout {
			return errors.New("the plugin did not send the converted animation back in time")
		}
		return nil
	}

	if now.Sub(e.enqueued) > pluginSilence && now.Sub(b.lastPoll) > pluginSilence {
		return errors.New("the Studio plugin is not asking for conversions (is Studio open with the CassFr plugin and HTTP requests allowed?)")
	}
	return nil
}

func (b *Broker) forget(id int64) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	delete(b.inflight, id)
	for i, e := range b.pending {
		if e.ID == id {
			b.pending = append(b.pending[:i], b.pending[i+1:]...)
			break
		}
	}
}

// Next hands out up to maxJobs queued jobs (stopping early once maxBytes of data is reached,
// but always returning at least one if any are queued). Called by the plugin.
func (b *Broker) Next(maxJobs, maxBytes int) []Job {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.lastPoll = time.Now()

	jobs := make([]Job, 0, maxJobs)
	size := 0
	taken := 0
	for taken < len(b.pending) && len(jobs) < maxJobs {
		e := b.pending[taken]
		if len(jobs) > 0 && size+len(e.Data) > maxBytes {
			break
		}

		size += len(e.Data)
		e.sentAt = time.Now()
		b.inflight[e.ID] = e
		jobs = append(jobs, e.Job)
		e.Data = "" // the plugin has it now, free the memory
		taken++
	}
	b.pending = b.pending[taken:]

	return jobs
}

// Complete delivers the plugin's answer to whoever is waiting in Convert.
func (b *Broker) Complete(r Result) {
	b.mutex.Lock()
	e, ok := b.inflight[r.ID]
	delete(b.inflight, r.ID)
	b.mutex.Unlock()

	if ok {
		e.done <- &r
	}
}
