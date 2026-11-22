package progress

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	StateIdle     = "idle"
	StateRunning  = "running"
	StateWaiting  = "waiting"
	StateFinished = "finished"
	StateError    = "error"

	ChunkPending = "pending"
	ChunkRunning = "running"
	ChunkDone    = "done"

	maxLogLines = 400
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type Chunk struct {
	Index      int    `json:"index"`
	IDs        int    `json:"ids"`
	Processed  int    `json:"processed"`
	Reuploaded int    `json:"reuploaded"`
	State      string `json:"state"`
}

type LogLine struct {
	Time  int64  `json:"t"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

type Status struct {
	State      string    `json:"state"`
	AssetType  string    `json:"assetType"`
	Message    string    `json:"message"`
	TotalIDs   int       `json:"totalIds"`
	DoneIDs    int       `json:"doneIds"`
	Reuploaded int       `json:"reuploaded"`
	Resumed    int       `json:"resumed"`
	UploadRate int       `json:"uploadRate"`
	InfoRate   int       `json:"infoRate"`
	RateLimits int       `json:"rateLimits"`
	ChunkIndex int       `json:"chunkIndex"`
	ChunkCount int       `json:"chunkCount"`
	Chunks     []Chunk   `json:"chunks"`
	StartedAt  int64     `json:"startedAt"`
	ElapsedMs  int64     `json:"elapsedMs"`
	Logs       []LogLine `json:"logs"`
	Version    string    `json:"version"`
}

type Tracker struct {
	mutex      sync.Mutex
	state      string
	assetType  string
	message    string
	totalIDs   int
	chunkIndex int
	resumed    int
	uploadRate int
	infoRate   int
	rateLimits int
	chunks     []Chunk
	startedAt  time.Time
	finishedAt time.Time
	logs       []LogLine
	version    string
}

var Global = &Tracker{state: StateIdle}

func (t *Tracker) SetVersion(v string) {
	t.mutex.Lock()
	t.version = v
	t.mutex.Unlock()
}

func (t *Tracker) Begin(assetType string, totalIDs int, chunkSizes []int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.state = StateRunning
	t.assetType = assetType
	t.message = ""
	t.totalIDs = totalIDs
	t.chunkIndex = 0
	t.resumed = 0
	t.rateLimits = 0
	t.startedAt = time.Now()
	t.finishedAt = time.Time{}
	t.logs = t.logs[:0]

	t.chunks = make([]Chunk, len(chunkSizes))
	for i, size := range chunkSizes {
		t.chunks[i] = Chunk{Index: i + 1, IDs: size, State: ChunkPending}
	}
}

func (t *Tracker) StartChunk(index, ids int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	for len(t.chunks) < index {
		t.chunks = append(t.chunks, Chunk{Index: len(t.chunks) + 1, State: ChunkPending})
	}

	chunk := &t.chunks[index-1]
	chunk.IDs = ids
	chunk.Processed = 0
	chunk.Reuploaded = 0
	chunk.State = ChunkRunning

	if t.totalIDs < ids {
		t.totalIDs = ids
	}

	t.chunkIndex = index
	t.state = StateRunning
}

func (t *Tracker) FinishChunk() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.chunkIndex < 1 || t.chunkIndex > len(t.chunks) {
		return
	}

	chunk := &t.chunks[t.chunkIndex-1]
	chunk.State = ChunkDone
	if chunk.Processed < chunk.IDs {
		chunk.Processed = chunk.IDs
	}

	if t.chunkIndex < len(t.chunks) {
		t.state = StateWaiting
	}
}

func (t *Tracker) Finish() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.state = StateFinished
	t.finishedAt = time.Now()
}

func (t *Tracker) Fail(message string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.state = StateError
	t.message = message
	t.finishedAt = time.Now()
}

func (t *Tracker) AddProcessed(n int) {
	if n == 0 {
		return
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.chunkIndex < 1 || t.chunkIndex > len(t.chunks) {
		return
	}
	t.chunks[t.chunkIndex-1].Processed += n
}

func (t *Tracker) SetUploadRate(rate int) {
	t.mutex.Lock()
	t.uploadRate = rate
	t.mutex.Unlock()
}

func (t *Tracker) SetInfoRate(rate int) {
	t.mutex.Lock()
	t.infoRate = rate
	t.mutex.Unlock()
}

func (t *Tracker) AddRateLimit() {
	t.mutex.Lock()
	t.rateLimits++
	t.mutex.Unlock()
}

func (t *Tracker) AddResumed(n int) {
	t.mutex.Lock()
	t.resumed += n
	t.mutex.Unlock()
}

func (t *Tracker) AddReuploaded(n int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.chunkIndex < 1 || t.chunkIndex > len(t.chunks) {
		return
	}
	t.chunks[t.chunkIndex-1].Reuploaded += n
}

func (t *Tracker) Log(level, msg string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.logs = append(t.logs, LogLine{
		Time:  time.Now().UnixMilli(),
		Level: level,
		Msg:   msg,
	})
	if over := len(t.logs) - maxLogLines; over > 0 {
		t.logs = append(t.logs[:0], t.logs[over:]...)
	}
}

func (t *Tracker) Snapshot() Status {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	status := Status{
		State:      t.state,
		AssetType:  t.assetType,
		Message:    t.message,
		TotalIDs:   t.totalIDs,
		ChunkIndex: t.chunkIndex,
		Resumed:    t.resumed,
		UploadRate: t.uploadRate,
		InfoRate:   t.infoRate,
		RateLimits: t.rateLimits,
		ChunkCount: len(t.chunks),
		Chunks:     make([]Chunk, len(t.chunks)),
		Logs:       make([]LogLine, len(t.logs)),
		Version:    t.version,
	}
	copy(status.Chunks, t.chunks)
	copy(status.Logs, t.logs)

	for _, chunk := range t.chunks {
		status.DoneIDs += chunk.Processed
		status.Reuploaded += chunk.Reuploaded
	}

	if !t.startedAt.IsZero() {
		status.StartedAt = t.startedAt.UnixMilli()
		end := time.Now()
		if !t.finishedAt.IsZero() {
			end = t.finishedAt
		}
		status.ElapsedMs = end.Sub(t.startedAt).Milliseconds()
	}

	return status
}

type Counter struct {
	n atomic.Int32
}

func (c *Counter) Add(delta int32) int32 {
	Global.AddProcessed(int(delta))
	return c.n.Add(delta)
}

func (c *Counter) Load() int32 {
	return c.n.Load()
}

type LogWriter struct{}

func (LogWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(string(p), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		level := "plain"
		switch {
		case strings.Contains(line, "\x1b[31m"):
			level = "error"
		case strings.Contains(line, "\x1b[32m"):
			level = "success"
		case strings.Contains(line, "\x1b[33m"):
			level = "warn"
		case strings.Contains(line, "\x1b[34m"):
			level = "info"
		}

		Global.Log(level, strings.TrimSpace(ansiPattern.ReplaceAllString(line, "")))
	}

	return len(p), nil
}
