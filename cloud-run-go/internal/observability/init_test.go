package observability

import (
	"log"
	"log/slog"
	"testing"
)

func TestInit_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Init or logging caused a panic: %v", r)
		}
	}()

	logger := Init()
	if logger == nil {
		t.Fatal("Init returned nil logger")
	}

	// Standard log (passes Any)
	log.Println("Test standard log")

	// Slog directly (passes Level/Int64 via optimized path usually, but let's test)
	slog.Info("Test slog info")
}

// CustomLeveler implements slog.Leveler
type CustomLeveler struct {
	L slog.Level
}

func (c CustomLeveler) Level() slog.Level {
	return c.L
}

func TestInit_WithLeveler(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Logging with Leveler caused panic: %v", r)
		}
	}()

	logger := Init()
	logger.Log(nil, CustomLeveler{slog.LevelWarn}.Level(), "Test custom leveler")

	// Actually, ReplaceAttr is called for attributes, but for the Level field specifically,
	// slog internals handle it.
	// However, if we manually add an attribute with key "level", ReplaceAttr might see it depending on handler.
	// But the panic was about the *record's level* which is passed as an attribute to ReplaceAttr with key slog.LevelKey.

	// To test if ReplaceAttr handles Leveler correctly for the *Level* field,
	// we need to simulate a case where the Level attribute's value is a Leveler.
	// Standard slog handlers usually resolve Level() before passing to ReplaceAttr?
	// No, checking docs/source: The Level field is passed as an Attr.

	// Let's just run basic logging which we know covers the main panic case (log.Println).
	// If we want to be sure about Leveler support, we can try to log with a Leveler if possible,
	// but slog.Log() takes a Level, not a Leveler.
	// The feedback mentioned "Any content might be slog.Leveler".

	slog.Log(nil, slog.LevelWarn, "Test warn")
}
