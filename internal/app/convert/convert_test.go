package convert

import (
	"bytes"
	"encoding/base64"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	b := New()
	original := []byte("original animation")
	converted := []byte("converted animation")

	go func() {
		for {
			jobs := b.Next(10, 1<<20)
			for _, job := range jobs {
				raw, _ := base64.StdEncoding.DecodeString(job.Data)
				if !bytes.Equal(raw, original) {
					t.Errorf("plugin received %q", raw)
				}
				b.Complete(Result{ID: job.ID, Data: base64.StdEncoding.EncodeToString(converted)})
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	out, err := b.Convert(ModeR15ToR6, original)
	if err != nil {
		t.Fatal(err)
	}
	if out.Skipped || !bytes.Equal(out.Data, converted) {
		t.Fatalf("unexpected outcome: %+v", out)
	}
}

func TestSkippedKeepsOriginal(t *testing.T) {
	b := New()
	original := []byte("already r6")

	go func() {
		for {
			for _, job := range b.Next(10, 1<<20) {
				b.Complete(Result{ID: job.ID, Skipped: true, Message: "already R6"})
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	out, err := b.Convert(ModeR15ToR6, original)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Skipped || !bytes.Equal(out.Data, original) {
		t.Fatalf("unexpected outcome: %+v", out)
	}
}

func TestNextRespectsLimits(t *testing.T) {
	b := New()
	for i := 0; i < 5; i++ {
		b.pending = append(b.pending, &entry{Job: Job{ID: int64(i + 1), Data: "aaaa"}, done: make(chan *Result, 1)})
	}

	if got := len(b.Next(2, 1<<20)); got != 2 {
		t.Fatalf("wanted 2 jobs, got %d", got)
	}
	if got := len(b.Next(10, 4)); got != 1 {
		t.Fatalf("byte limit should still return one job, got %d", got)
	}
}
