package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kartFr/Asset-Reuploader/internal/app/assets"
	"github.com/kartFr/Asset-Reuploader/internal/app/checkpoint"
	"github.com/kartFr/Asset-Reuploader/internal/app/config"
	"github.com/kartFr/Asset-Reuploader/internal/app/convert"
	"github.com/kartFr/Asset-Reuploader/internal/app/dashboard"
	"github.com/kartFr/Asset-Reuploader/internal/app/progress"
	"github.com/kartFr/Asset-Reuploader/internal/app/request"
	"github.com/kartFr/Asset-Reuploader/internal/app/response"
	"github.com/kartFr/Asset-Reuploader/internal/color"
	"github.com/kartFr/Asset-Reuploader/internal/files"
	"github.com/kartFr/Asset-Reuploader/internal/roblox"
)

var CompatiblePluginVersion = ""

const flushInterval = 2 * time.Second

type handlerFactory func(assetType string, c *roblox.Client, r *request.RawRequest, resp *response.Response) (func() error, error)

func getOutputFileName(reuploadType string) string {
	t := time.Now()
	return fmt.Sprintf("Output_%s_%s.json", reuploadType, t.Format("2006-01-02_15-04-05"))
}

func isBrowser(r *http.Request) bool {
	return strings.Contains(r.UserAgent(), "Mozilla") && strings.Contains(r.Header.Get("Accept"), "text/html")
}

func serve(c *roblox.Client) error {
	progress.Global.SetVersion("CassFr " + Version)
	if err := checkpoint.Global.Load(); err != nil {
		color.Error.Println("Could not read cassfr-progress.json, saved progress will not be used: ", err)
	}

	url := "http://localhost:" + port + "/ui"
	openDashboard := func() {
		if strings.EqualFold(strings.TrimSpace(config.Get("open_dashboard")), "false") {
			return
		}
		dashboard.Open(url)
	}

	return http.ListenAndServe(":"+port, newMux(c, assets.NewReuploadHandlerWithType, openDashboard))
}

func newMux(c *roblox.Client, newHandler handlerFactory, openDashboard func()) *http.ServeMux {
	var mutex sync.Mutex
	var busy bool
	var startFailed bool
	var sessionActive bool
	var sessionIndex, sessionCount int
	var dashboardOpened bool
	finished := true

	var historyMutex sync.Mutex
	var exportedJSONName string
	var exportJSON bool
	var lastFlush time.Time
	var resumeKey string
	respHistory := make([]response.ResponseItem, 0)

	flushHistory := func(force bool) {
		historyMutex.Lock()
		defer historyMutex.Unlock()

		if !exportJSON || len(respHistory) == 0 {
			return
		}
		if !force && time.Since(lastFlush) < flushInterval {
			return
		}

		j, err := json.Marshal(respHistory)
		if err != nil {
			log.Fatal(err)
		}

		if err := files.Write(exportedJSONName, string(j)); err != nil {
			log.Fatal(err)
		}
		lastFlush = time.Now()
	}

	resetHistory := func() {
		historyMutex.Lock()
		exportJSON = false
		respHistory = make([]response.ResponseItem, 0)
		historyMutex.Unlock()
	}

	flushAll := func(force bool) {
		flushHistory(force)
		if force {
			checkpoint.Global.Flush()
		}
	}

	resp := response.New(func(i response.ResponseItem) {
		historyMutex.Lock()
		key := resumeKey
		if exportJSON {
			respHistory = append(respHistory, i)
		}
		historyMutex.Unlock()

		if key != "" {
			if known, ok := checkpoint.Global.Lookup(key, i.OldID); ok && known == i.NewID {
				flushHistory(false)
				return
			}
			checkpoint.Global.Add(key, i.OldID, i.NewID)
		}

		progress.Global.AddReuploaded(1)
		flushHistory(false)
	})

	mux := http.NewServeMux()

	mux.HandleFunc("GET /ui", dashboard.Page)
	mux.HandleFunc("GET /status", dashboard.Status)
	mux.HandleFunc("GET /settings", dashboard.GetSettings)
	mux.HandleFunc("POST /settings", dashboard.PostSettings)
	mux.HandleFunc("POST /settings/clear-progress", dashboard.ClearProgress)

	mux.HandleFunc("GET /convert/next", func(w http.ResponseWriter, r *http.Request) {
		maxJobs := 40
		if n, err := strconv.Atoi(r.URL.Query().Get("max")); err == nil && n > 0 && n < maxJobs {
			maxJobs = n
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(convert.Global.Next(maxJobs, 1_500_000)); err != nil {
			log.Println("convert/next:", err)
		}
	})

	mux.HandleFunc("POST /convert/result", func(w http.ResponseWriter, r *http.Request) {
		var results []convert.Result
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&results); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		for _, result := range results {
			convert.Global.Complete(result)
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if isBrowser(r) {
			http.Redirect(w, r, "/ui", http.StatusFound)
			return
		}

		mutex.Lock()
		defer mutex.Unlock()

		w.WriteHeader(http.StatusOK)
		if resp.Len() == 0 && !busy {
			if startFailed {
				startFailed = false
				fmt.Fprint(w, "failed")
				return
			}

			if finished {
				return
			}
			finished = true

			if sessionActive && sessionIndex < sessionCount {
				flushAll(true)
				fmt.Fprint(w, "done")
				fmt.Printf("Chunk %d/%d finished. Waiting for the plugin to send chunk %d...\n", sessionIndex, sessionCount, sessionIndex+1)
				return
			}

			flushAll(true)
			resetHistory()
			sessionActive = false
			progress.Global.Finish()

			fmt.Fprint(w, "done")
			fmt.Println("Finished reuploading. (you can rerun without restarting)")
			return
		}

		if err := resp.EncodeJSONAndClear(json.NewEncoder(w)); err != nil {
			log.Fatal(err)
		}
	})

	mux.HandleFunc("POST /reupload", func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()

		if busy || !finished {
			mutex.Unlock()
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		var req request.RawRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			mutex.Unlock()
			color.Error.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if CompatiblePluginVersion != "" && req.PluginVersion != CompatiblePluginVersion {
			mutex.Unlock()
			w.WriteHeader(http.StatusConflict)
			return
		}

		chunkIndex, chunkCount := req.ChunkIndex, req.ChunkCount
		if chunkCount < 1 {
			chunkIndex, chunkCount = 1, 1
		}

		continuing := chunkIndex > 1 && sessionActive && sessionIndex == chunkIndex-1 && sessionCount == chunkCount
		if chunkIndex < 1 || chunkIndex > chunkCount || (!continuing && chunkIndex != 1) {
			mutex.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if exists := assets.DoesModuleExist(req.AssetType); !exists {
			mutex.Unlock()
			w.WriteHeader(http.StatusNotFound)
			return
		}

		originalIDs := len(req.IDs)
		var resumeKeyForChunk string
		var knownItems []response.ResponseItem
		if config.Get("resume_progress") != "false" && req.AssetType != "Sound" {
			// Converted animations get their own saved progress so a converted run never reuses
			// ids from an unconverted one (or from the other conversion direction).
			keyType := req.AssetType
			if req.AssetType == "Animation" && convert.ValidMode(req.AnimationConvert) {
				keyType += "+" + req.AnimationConvert
			}
			resumeKeyForChunk = checkpoint.Key(keyType, req.IsGroup, req.CreatorID)
			remaining := make([]int64, 0, len(req.IDs))
			for _, id := range req.IDs {
				if newID, ok := checkpoint.Global.Lookup(resumeKeyForChunk, id); ok {
					knownItems = append(knownItems, response.ResponseItem{OldID: id, NewID: newID})
					continue
				}
				remaining = append(remaining, id)
			}
			req.IDs = remaining
		}

		startReupload, err := newHandler(req.AssetType, c, &req, resp)
		if err != nil {
			mutex.Unlock()
			color.Error.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !continuing {
			historyMutex.Lock()
			exportJSON = req.ExportJSON
			if exportJSON {
				exportedJSONName = getOutputFileName(req.AssetType)
			}
			respHistory = make([]response.ResponseItem, 0)
			historyMutex.Unlock()

			sizes := req.ChunkSizes
			if len(sizes) != chunkCount {
				sizes = make([]int, chunkCount)
				sizes[0] = len(req.IDs)
			}

			total := req.TotalIDs
			if total < len(req.IDs) {
				total = len(req.IDs)
			}

			sessionActive = true
			sessionCount = chunkCount
			progress.Global.Begin(req.AssetType, total, sizes)
		}

		sessionIndex = chunkIndex
		progress.Global.StartChunk(chunkIndex, originalIDs)

		historyMutex.Lock()
		resumeKey = resumeKeyForChunk
		historyMutex.Unlock()

		if len(knownItems) > 0 {
			progress.Global.AddProcessed(len(knownItems))
			progress.Global.AddResumed(len(knownItems))
			progress.Global.Log("info", fmt.Sprintf("Saved progress: %d of %d ids in this chunk were already reuploaded earlier, applying them without uploading again.", len(knownItems), originalIDs))
			for _, item := range knownItems {
				resp.AddItem(item)
			}
		}

		busy = true
		finished = false
		startFailed = false

		shouldOpen := !dashboardOpened
		dashboardOpened = true
		mutex.Unlock()

		if shouldOpen {
			go openDashboard()
		}

		go func() {
			start := time.Now()
			err := startReupload()

			mutex.Lock()
			busy = false
			if err != nil {
				finished = true
				startFailed = true
				sessionActive = false
			}
			mutex.Unlock()

			if err != nil {
				progress.Global.Fail(err.Error())
				flushAll(true)
				resetHistory()
				color.Error.Println("Failed to start reuploading: ", err)
				return
			}

			progress.Global.FinishChunk()
			flushAll(true)

			duration := time.Since(start)
			fmt.Printf("Chunk %d/%d took %d hours, %d minutes, and %d seconds\n", chunkIndex, chunkCount, int(duration.Hours()), int(duration.Minutes())%60, int(duration.Seconds())%60)
			fmt.Println("Waiting for client to finish changing ids...")
		}()

		w.WriteHeader(http.StatusOK)
	})

	return mux
}
