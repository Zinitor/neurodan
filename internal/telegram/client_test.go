package telegram_test

import (
	"os"
	"testing"

	"github.com/aeremin/neurodan/internal/telegram"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	if len(os.Getenv("BOT_TOKEN")) == 0 {
		t.Skip("skipping: BOT_TOKEN not set")
	}
	client := telegram.NewClient("test-token", "http://localhost:8000")
	require.NotNil(t, client)
}