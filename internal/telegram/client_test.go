package telegram_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aeremin/neurodan/internal/telegram"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	if len(os.Getenv("BOT_TOKEN")) == 0 {
		t.Skip("skipping: BOT_TOKEN not set")
	}
	store, err := telegram.NewSettingsStore(filepath.Join(t.TempDir(), "s.json"))
	require.NoError(t, err)
	client := telegram.NewClient("test-token", "http://localhost:8000", telegram.Config{}, store)
	require.NotNil(t, client)
}
