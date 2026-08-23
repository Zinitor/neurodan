package telegram

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAnecdoteArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        string
		wantStylize bool
		wantSeed    string
	}{
		{"no args", "", false, ""},
		{"seed only", "привет мир", false, "привет мир"},
		{"s flag with seed", "s привет мир", true, "привет мир"},
		{"bare s flag", "s", true, ""},
		{"s flag uppercase", "S привет", true, "привет"},
		{"s flag surrounded by spaces", "  s   привет  ", true, "привет"},
		{"s flag not at start keeps seed", "пока s миру", false, "пока s миру"},
		{"multiple spaces collapse", "привет    мир", false, "привет мир"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStylize, gotSeed := parseAnecdoteArgs(tt.args)
			require.Equal(t, tt.wantStylize, gotStylize)
			require.Equal(t, tt.wantSeed, gotSeed)
		})
	}
}

func TestIsCommandText(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"/anec", true},
		{"/anec привет мир", true},
		{"/start@MyBot", true},
		{"  /anec leading spaces", true},
		{"", false},
		{"привет мир", false},
		{"50% скидка на /anec только сегодня", false},
		{"просто текст", false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			require.Equal(t, tt.want, isCommandText(tt.text))
		})
	}
}
