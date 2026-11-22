package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kartFr/Asset-Reuploader/internal/app/checkpoint"
	"github.com/kartFr/Asset-Reuploader/internal/app/progress"
	"github.com/kartFr/Asset-Reuploader/internal/app/request"
	"github.com/kartFr/Asset-Reuploader/internal/app/response"
	"github.com/kartFr/Asset-Reuploader/internal/roblox"
)

func fakeFactory(release chan struct{}) handlerFactory {
	return func(assetType string, c *roblox.Client, r *request.RawRequest, resp *response.Response) (func() error, error) {
		return func() error {
			if release != nil {
				<-release
			}
			for _, id := range r.IDs {
				resp.AddItem(response.ResponseItem{OldID: id, NewID: id + 1000})
			}
			return nil
		}, nil
	}
}

func post(t *testing.T, srv *httptest.Server, body map[string]any) int {
	t.Helper()
	j, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/reupload", "application/json", strings.NewReader(string(j)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func poll(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return string(b)
}

func pollUntilDone(t *testing.T, srv *httptest.Server) []response.ResponseItem {
	t.Helper()
	var all []response.ResponseItem
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body := poll(t, srv)
		if body == "done" {
			return all
		}
		if body == "failed" {
			t.Fatal("got failed")
		}
		var items []response.ResponseItem
		if json.Unmarshal([]byte(body), &items) == nil {
			all = append(all, items...)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for done")
	return nil
}

func TestChunkedSession(t *testing.T) {
	srv := httptest.NewServer(newMux(nil, fakeFactory(nil), func() {}))
	defer srv.Close()

	base := func(index int, ids []int) map[string]any {
		return map[string]any{
			"placeId": 1, "creatorId": 1, "ids": ids, "defaultPlaceIds": []int{},
			"pluginVersion": "x", "assetType": "Sound", "exportJson": false, "isGroup": false,
			"chunkIndex": index, "chunkCount": 3, "chunkSizes": []int{3, 2, 1}, "totalIds": 6,
		}
	}

	if code := post(t, srv, base(2, []int{9})); code != http.StatusBadRequest {
		t.Fatalf("out of order chunk should be 400, got %d", code)
	}

	var got []response.ResponseItem
	chunks := [][]int{{1, 2, 3}, {4, 5}, {6}}
	for i, ids := range chunks {
		if code := post(t, srv, base(i+1, ids)); code != http.StatusOK {
			t.Fatalf("chunk %d post got %d", i+1, code)
		}
		got = append(got, pollUntilDone(t, srv)...)
	}

	if len(got) != 6 {
		t.Fatalf("expected 6 replaced ids across chunks, got %d", len(got))
	}

	s := progress.Global.Snapshot()
	if s.State != progress.StateFinished || s.ChunkCount != 3 || s.Reuploaded != 6 || s.TotalIDs != 6 {
		t.Fatalf("bad final status: %+v", s)
	}
	for _, c := range s.Chunks {
		if c.State != progress.ChunkDone {
			t.Fatalf("chunk not done: %+v", c)
		}
	}

	if body := poll(t, srv); body != "" {
		t.Fatalf("expected idle empty response, got %q", body)
	}

	if code := post(t, srv, base(1, []int{1})); code != http.StatusOK {
		t.Fatalf("new session after finish should start fine, got %d", code)
	}
}

func TestBusyAndNoDoneBeforeChunkEnds(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(newMux(nil, fakeFactory(release), func() {}))
	defer srv.Close()

	body := map[string]any{
		"ids": []int{1, 2}, "pluginVersion": "x", "assetType": "Sound",
		"chunkIndex": 1, "chunkCount": 2, "chunkSizes": []int{2, 1}, "totalIds": 3,
	}
	if code := post(t, srv, body); code != http.StatusOK {
		t.Fatalf("got %d", code)
	}
	if code := post(t, srv, body); code != http.StatusServiceUnavailable {
		t.Fatalf("second post while busy should be 503, got %d", code)
	}
	if b := poll(t, srv); b == "done" {
		t.Fatal("done sent while chunk still running")
	}
	close(release)
	pollUntilDone(t, srv)
}

func TestLegacyRequestWithoutChunkFields(t *testing.T) {
	srv := httptest.NewServer(newMux(nil, fakeFactory(nil), func() {}))
	defer srv.Close()

	if code := post(t, srv, map[string]any{"ids": []int{1, 2}, "pluginVersion": "x", "assetType": "Sound"}); code != http.StatusOK {
		t.Fatalf("got %d", code)
	}
	pollUntilDone(t, srv)
	if s := progress.Global.Snapshot(); s.State != progress.StateFinished || s.ChunkCount != 1 {
		t.Fatalf("bad status %+v", s)
	}
}

func TestBrowserPollDoesNotDrain(t *testing.T) {
	srv := httptest.NewServer(newMux(nil, fakeFactory(nil), func() {}))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("browser should be redirected, got %d", res.StatusCode)
	}

	res, _ = http.Get(srv.URL + "/favicon.ico")
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("favicon must not hit the poll handler, got %d", res.StatusCode)
	}

	res, _ = http.Get(srv.URL + "/status")
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(b), `"state"`) {
		t.Fatalf("bad status body %s", b)
	}
}

func TestSavedProgressIsSkippedAndApplied(t *testing.T) {
	checkpoint.Global.SetPath(filepath.Join(t.TempDir(), "progress.json"))
	key := checkpoint.Key("Animation", false, 1)
	checkpoint.Global.Add(key, 10, 100)
	t.Cleanup(func() { checkpoint.Global.Clear() })

	var seen []int64
	factory := func(assetType string, c *roblox.Client, r *request.RawRequest, resp *response.Response) (func() error, error) {
		seen = append([]int64(nil), r.IDs...)
		return func() error {
			for _, id := range r.IDs {
				resp.AddItem(response.ResponseItem{OldID: id, NewID: id + 1000})
			}
			return nil
		}, nil
	}

	srv := httptest.NewServer(newMux(nil, factory, func() {}))
	defer srv.Close()

	code := post(t, srv, map[string]any{
		"ids": []int64{10, 11}, "creatorId": 1, "isGroup": false,
		"pluginVersion": "x", "assetType": "Animation", "totalIds": 2,
	})
	if code != http.StatusOK {
		t.Fatalf("got %d", code)
	}

	items := pollUntilDone(t, srv)
	if len(seen) != 1 || seen[0] != 11 {
		t.Fatalf("uploader should only see the new id, saw %v", seen)
	}

	got := map[int64]int64{}
	for _, item := range items {
		got[item.OldID] = item.NewID
	}
	if got[10] != 100 || got[11] != 1011 || len(got) != 2 {
		t.Fatalf("plugin should receive both ids, got %v", got)
	}
	if id, ok := checkpoint.Global.Lookup(key, 11); !ok || id != 1011 {
		t.Fatalf("new upload should be saved, got %d %v", id, ok)
	}

	s := progress.Global.Snapshot()
	if s.Resumed != 1 || s.Reuploaded != 1 || s.DoneIDs != 2 {
		t.Fatalf("bad counters: resumed=%d reuploaded=%d done=%d", s.Resumed, s.Reuploaded, s.DoneIDs)
	}
}

func TestSettingsEndpointRejectsOutsideRequests(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	srv := httptest.NewServer(newMux(nil, fakeFactory(nil), func() {}))
	defer srv.Close()

	body := `{"preset":"turbo","pollMs":250,"resumeProgress":true,"openDashboard":false}`
	send := func(headers map[string]string) int {
		req, _ := http.NewRequest("POST", srv.URL+"/settings", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}

	if code := send(nil); code != http.StatusForbidden {
		t.Fatalf("missing header should be 403, got %d", code)
	}
	if code := send(map[string]string{"X-CassFr": "1", "Origin": "https://evil.example"}); code != http.StatusForbidden {
		t.Fatalf("foreign origin should be 403, got %d", code)
	}
	if code := send(map[string]string{"X-CassFr": "1"}); code != http.StatusOK {
		t.Fatalf("valid request should be 200, got %d", code)
	}

	saved, err := os.ReadFile(filepath.Join(dir, "config.ini"))
	if err != nil || !strings.Contains(string(saved), "preset=turbo") || !strings.Contains(string(saved), "live_poll_ms=250") {
		t.Fatalf("settings not saved to config.ini: %v %s", err, saved)
	}

	bad := strings.NewReader(`{"preset":"custom","infoRateStart":900,"infoRateMax":100,"animationRateStart":10,"animationRateMax":20,"animationConcurrency":5,"animationParallelBatch":5,"pollMs":500}`)
	req, _ := http.NewRequest("POST", srv.URL+"/settings", bad)
	req.Header.Set("X-CassFr", "1")
	res, _ := http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("start above max should be 400, got %d", res.StatusCode)
	}
}
