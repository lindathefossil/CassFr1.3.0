package dashboard

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/kartFr/Asset-Reuploader/internal/app/checkpoint"
	"github.com/kartFr/Asset-Reuploader/internal/app/progress"
	"github.com/kartFr/Asset-Reuploader/internal/app/settings"
)

//go:embed index.html
var page []byte

func Page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(page)
}

func Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(progress.Global.Snapshot())
}

func Open(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

type settingsResponse struct {
	Settings   settings.Settings            `json:"settings"`
	Presets    map[string]settings.Settings `json:"presets"`
	SavedCount int                          `json:"savedCount"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func allowedWrite(r *http.Request) bool {
	return r.Header.Get("X-CassFr") == "1" && sameOrigin(r)
}

func GetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsResponse{
		Settings:   settings.Current(),
		Presets:    settings.Presets,
		SavedCount: checkpoint.Global.Count(),
	})
}

func PostSettings(w http.ResponseWriter, r *http.Request) {
	if !allowedWrite(r) {
		writeError(w, http.StatusForbidden, "request not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var incoming settings.Settings
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		writeError(w, http.StatusBadRequest, "invalid settings")
		return
	}

	applied, err := settings.Apply(incoming)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, settingsResponse{
		Settings:   applied,
		Presets:    settings.Presets,
		SavedCount: checkpoint.Global.Count(),
	})
}

func ClearProgress(w http.ResponseWriter, r *http.Request) {
	if !allowedWrite(r) {
		writeError(w, http.StatusForbidden, "request not allowed")
		return
	}

	removed := checkpoint.Global.Clear()
	writeJSON(w, http.StatusOK, map[string]int{"removed": removed})
}
