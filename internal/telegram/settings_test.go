package telegram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettingsSetAndGet(t *testing.T) {
	store, err := NewSettingsStore(filepath.Join(t.TempDir(), "s.json"))
	require.NoError(t, err)

	require.True(t, store.IsEnabled(1, FeatureSwear), "features default to enabled")
	require.True(t, store.IsEnabled(1, FeatureIntel))

	require.NoError(t, store.Set(1, FeatureSwear, false))
	require.False(t, store.IsEnabled(1, FeatureSwear))
	require.True(t, store.IsEnabled(1, FeatureIntel), "other features unaffected")

	// Other chats unaffected.
	require.True(t, store.IsEnabled(2, FeatureSwear))
}

func TestSettingsPersistAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "chat_settings.json")

	store, err := NewSettingsStore(path)
	require.NoError(t, err)
	require.NoError(t, store.Set(42, FeatureSwear, false))
	require.NoError(t, store.Set(42, FeatureCaps, false))

	// Simulate a restart: load a fresh store from the same file.
	reloaded, err := NewSettingsStore(path)
	require.NoError(t, err)
	require.False(t, reloaded.IsEnabled(42, FeatureSwear))
	require.False(t, reloaded.IsEnabled(42, FeatureCaps))
	require.True(t, reloaded.IsEnabled(42, FeatureIntel), "untouched features stay on")
	require.True(t, reloaded.IsEnabled(999, FeatureSwear), "other chats default on")
}

func TestSettingsMissingFile(t *testing.T) {
	store, err := NewSettingsStore(filepath.Join(t.TempDir(), "does-not-exist.json"))
	require.NoError(t, err)
	require.True(t, store.IsEnabled(1, FeatureSwear))
}

func TestSettingsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))
	store, err := NewSettingsStore(path)
	require.NoError(t, err)
	require.True(t, store.IsEnabled(1, FeaturePunct))
}

func TestSettingsSummaryShowsToggles(t *testing.T) {
	store, err := NewSettingsStore(filepath.Join(t.TempDir(), "s.json"))
	require.NoError(t, err)
	require.NoError(t, store.Set(7, FeatureSwear, false))

	summary := store.Summary(7)
	require.Contains(t, summary, "swearify")
	require.Contains(t, summary, "❌")
	require.Contains(t, summary, "✅")
}

func TestNormalizeFeature(t *testing.T) {
	require.Equal(t, "allcaps", normalizeFeature("All-Caps"))
	require.Equal(t, "allcaps", normalizeFeature("ALL_CAPS"))
	require.Equal(t, "allcaps", normalizeFeature(" all caps "))
	require.Equal(t, "swearify", normalizeFeature("swearify"))
}

func TestIsValidFeature(t *testing.T) {
	require.True(t, isValidFeature(FeatureSwear))
	require.True(t, isValidFeature(FeatureIntel))
	require.False(t, isValidFeature("nonsense"))
	require.False(t, isValidFeature(""))
	for _, f := range allFeatures {
		require.True(t, isValidFeature(f), "expected %q to be a valid feature", f)
	}
}

func TestFeatureDescriptionsAreRussian(t *testing.T) {
	for _, f := range allFeatures {
		desc, ok := featureDesc[f]
		require.True(t, ok, "missing description for %q", f)
		require.True(t, strings.TrimSpace(desc) != "", "empty description for %q", f)
	}
}
