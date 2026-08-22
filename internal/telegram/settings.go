package telegram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Feature names that can be toggled per chat via /on and /off.
const (
	FeatureCaps    = "caps"
	FeatureAllCaps = "allcaps"
	FeaturePunct   = "punct"
	FeatureEmotes  = "emotes"
	FeatureSwear   = "swearify"
	FeatureIntel   = "intellectualize"
)

// allFeatures lists every toggleable feature in help-display order.
var allFeatures = []string{FeatureCaps, FeatureAllCaps, FeaturePunct, FeatureEmotes, FeatureSwear, FeatureIntel}

// featureDesc maps a feature to its Russian help/status description.
var featureDesc = map[string]string{
	FeatureCaps:    "КАПС — заглавная буква в случайном слове",
	FeatureAllCaps: "allcaps — фраза ЦЕЛИКОМ КАПСОМ",
	FeaturePunct:   "punct — пунктуация (…, !, !!)",
	FeatureEmotes:  "emotes — реакции и каомодзи (😂, кек, лол)",
	FeatureSwear:   "swearify — мат",
	FeatureIntel:   "intellectualize — заумные слова",
}

// normalizeFeature lowercases and strips separators so "All-Caps", "ALL_CAPS"
// and "allcaps" all map to the same feature.
func normalizeFeature(f string) string {
	f = strings.ToLower(strings.TrimSpace(f))
	f = strings.ReplaceAll(f, "-", "")
	f = strings.ReplaceAll(f, "_", "")
	f = strings.ReplaceAll(f, " ", "")
	return f
}

func isValidFeature(f string) bool {
	for _, x := range allFeatures {
		if x == f {
			return true
		}
	}
	return false
}

// SettingsStore persists per-chat stylization feature toggles to a JSON file
// so they survive bot restarts. It is safe for concurrent use.
type SettingsStore struct {
	mu       sync.Mutex
	path     string
	features map[int64]map[string]bool
}

// NewSettingsStore loads (or initializes) the settings file at path.
func NewSettingsStore(path string) (*SettingsStore, error) {
	s := &SettingsStore{
		path:     path,
		features: map[int64]map[string]bool{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// IsEnabled reports whether the feature is on for the chat. Unknown features
// default to enabled.
func (s *SettingsStore) IsEnabled(chatID int64, feature string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.features[chatID][feature]; ok {
		return v
	}
	return true
}

// Set enables or disables a feature for a chat and persists the change.
func (s *SettingsStore) Set(chatID int64, feature string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.features[chatID] == nil {
		s.features[chatID] = map[string]bool{}
	}
	s.features[chatID][feature] = enabled
	return s.saveLocked()
}

// Summary renders a human-readable per-feature status for a chat.
func (s *SettingsStore) Summary(chatID int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var b strings.Builder
	b.WriteString("Текущие настройки:\n")
	for _, f := range allFeatures {
		state := "❌"
		if v, ok := s.features[chatID][f]; !ok || v {
			state = "✅"
		}
		fmt.Fprintf(&b, "%s %s\n", state, featureDesc[f])
	}
	b.WriteString("\n/on /off <фичи> — изменить (например: /off swearify allcaps)")
	return b.String()
}

func (s *SettingsStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read settings %s: %w", s.path, err)
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &s.features)
}

// saveLocked persists settings atomically (write temp + rename). Callers must
// hold s.mu.
func (s *SettingsStore) saveLocked() error {
	if dir := filepath.Dir(s.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir settings dir: %w", err)
		}
	}
	data, err := json.MarshalIndent(s.features, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
