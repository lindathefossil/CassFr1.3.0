package settings

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/kartFr/Asset-Reuploader/internal/app/config"
)

type Settings struct {
	Preset                 string `json:"preset"`
	InfoRateStart          int    `json:"infoRateStart"`
	InfoRateMax            int    `json:"infoRateMax"`
	AnimationRateStart     int    `json:"animationRateStart"`
	AnimationRateMax       int    `json:"animationRateMax"`
	AnimationConcurrency   int    `json:"animationConcurrency"`
	AnimationParallelBatch int    `json:"animationParallelBatch"`
	PollMs                 int    `json:"pollMs"`
	OpenDashboard          bool   `json:"openDashboard"`
	ResumeProgress         bool   `json:"resumeProgress"`
}

var Presets = map[string]Settings{
	"gentle": {
		Preset: "gentle", InfoRateStart: 200, InfoRateMax: 600,
		AnimationRateStart: 300, AnimationRateMax: 800,
		AnimationConcurrency: 24, AnimationParallelBatch: 6,
	},
	"balanced": {
		Preset: "balanced", InfoRateStart: 300, InfoRateMax: 900,
		AnimationRateStart: 600, AnimationRateMax: 1500,
		AnimationConcurrency: 48, AnimationParallelBatch: 10,
	},
	"turbo": {
		Preset: "turbo", InfoRateStart: 600, InfoRateMax: 1800,
		AnimationRateStart: 1200, AnimationRateMax: 3000,
		AnimationConcurrency: 96, AnimationParallelBatch: 16,
	},
}

func isTrue(value string) bool {
	return !strings.EqualFold(strings.TrimSpace(value), "false")
}

func Current() Settings {
	preset := strings.ToLower(strings.TrimSpace(config.Get("preset")))
	if preset == "" {
		preset = "balanced"
	}

	return Settings{
		Preset:                 preset,
		InfoRateStart:          config.GetInt("info_rate_start", 300),
		InfoRateMax:            config.GetInt("info_rate_max", 900),
		AnimationRateStart:     config.GetInt("animation_rate_start", 600),
		AnimationRateMax:       config.GetInt("animation_rate_max", 1500),
		AnimationConcurrency:   config.GetInt("animation_concurrency", 48),
		AnimationParallelBatch: config.GetInt("animation_parallel_batch", 10),
		PollMs:                 config.GetInt("live_poll_ms", 1000),
		OpenDashboard:          isTrue(config.Get("open_dashboard")),
		ResumeProgress:         isTrue(config.Get("resume_progress")),
	}
}

func inRange(name string, value, low, high int) error {
	if value < low || value > high {
		return fmt.Errorf("%s must be between %d and %d", name, low, high)
	}
	return nil
}

func Validate(s Settings) error {
	checks := []error{
		inRange("info rate start", s.InfoRateStart, 10, 20000),
		inRange("info rate max", s.InfoRateMax, 10, 20000),
		inRange("animation rate start", s.AnimationRateStart, 10, 20000),
		inRange("animation rate max", s.AnimationRateMax, 10, 20000),
		inRange("animation concurrency", s.AnimationConcurrency, 1, 500),
		inRange("parallel batches", s.AnimationParallelBatch, 1, 64),
		inRange("poll speed", s.PollMs, 100, 2000),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}

	if s.InfoRateStart > s.InfoRateMax {
		return errors.New("info rate start cannot be higher than its max")
	}
	if s.AnimationRateStart > s.AnimationRateMax {
		return errors.New("animation rate start cannot be higher than its max")
	}
	return nil
}

func Apply(s Settings) (Settings, error) {
	s.Preset = strings.ToLower(strings.TrimSpace(s.Preset))
	if preset, ok := Presets[s.Preset]; ok {
		s.InfoRateStart = preset.InfoRateStart
		s.InfoRateMax = preset.InfoRateMax
		s.AnimationRateStart = preset.AnimationRateStart
		s.AnimationRateMax = preset.AnimationRateMax
		s.AnimationConcurrency = preset.AnimationConcurrency
		s.AnimationParallelBatch = preset.AnimationParallelBatch
	} else {
		s.Preset = "custom"
	}

	if err := Validate(s); err != nil {
		return s, err
	}

	config.Set("preset", s.Preset)
	config.Set("info_rate_start", strconv.Itoa(s.InfoRateStart))
	config.Set("info_rate_max", strconv.Itoa(s.InfoRateMax))
	config.Set("animation_rate_start", strconv.Itoa(s.AnimationRateStart))
	config.Set("animation_rate_max", strconv.Itoa(s.AnimationRateMax))
	config.Set("animation_concurrency", strconv.Itoa(s.AnimationConcurrency))
	config.Set("animation_parallel_batch", strconv.Itoa(s.AnimationParallelBatch))
	config.Set("live_poll_ms", strconv.Itoa(s.PollMs))
	config.Set("open_dashboard", strconv.FormatBool(s.OpenDashboard))
	config.Set("resume_progress", strconv.FormatBool(s.ResumeProgress))

	return s, config.Save()
}
