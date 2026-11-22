package config

import (
	"bufio"
	"errors"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/kartFr/Asset-Reuploader/internal/files"
)

var (
	mutex         sync.RWMutex
	config        = make(map[string]string, 0)
	defaultConfig = map[string]string{
		"port":           "38073",
		"cookie_file":    "cookie.txt",
		"api_key":        "",
		"api_key_file":   "api-key.txt",
		"open_dashboard": "true",

		"info_rate_start":          "300",
		"info_rate_max":            "900",
		"animation_rate_start":     "600",
		"animation_rate_max":       "1500",
		"animation_concurrency":    "48",
		"animation_parallel_batch": "10",
		"preset":                   "balanced",
		"live_poll_ms":             "1000",
		"resume_progress":          "true",
	}
)

func init() {
	contents, err := files.Read("config.ini")
	if err != nil && !os.IsNotExist(err) {
		log.Printf("failed reading config.ini, using defaults: %v", err)
	}
	if err != nil {
		contents = ""
	}

	scanner := bufio.NewScanner(strings.NewReader(contents))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		split := strings.SplitN(line, "=", 2)
		if len(split) != 2 {
			continue
		}

		key := strings.TrimSpace(split[0])
		if key == "" {
			continue
		}
		config[key] = split[1]
	}

	for i, v := range defaultConfig {
		if _, exists := config[i]; exists {
			continue
		}
		config[i] = v
	}

	keyFile := config["api_key_file"]
	data, err := files.Read(keyFile)
	switch {
	case err == nil && strings.TrimSpace(data) != "":
		config["api_key"] = strings.TrimSpace(data)
	case err != nil && errors.Is(err, os.ErrNotExist):
		if k := strings.TrimSpace(config["api_key"]); k != "" {
			if wErr := files.Write(keyFile, k); wErr != nil {
				log.Printf("could not migrate api key to %s: %v", keyFile, wErr)
			}
		}
	}

}

// PersistAPIKey writes the current api_key to api-key_file (Open Cloud key). Call after Set("api_key", ...).
func PersistAPIKey() error {
	k := strings.TrimSpace(Get("api_key"))
	if k == "" {
		return nil
	}
	return files.Write(Get("api_key_file"), k)
}

func GetInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(Get(key)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func Get(key string) string {
	mutex.RLock()
	defer mutex.RUnlock()
	return config[key]
}

func Set(key string, value string) {
	mutex.Lock()
	defer mutex.Unlock()
	config[key] = value
}

func Save() error {
	mutex.RLock()
	defer mutex.RUnlock()

	var out strings.Builder
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if key == "api_key" {
			continue
		}
		out.WriteString(key)
		out.WriteByte('=')
		out.WriteString(config[key])
		out.WriteByte('\n')
	}
	return files.Write("config.ini", out.String())
}
