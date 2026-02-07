package observability

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strings"
)

// Init configures structured logging.
//
// Env:
// - LOG_FORMAT: "json" | "text" (default: text)
// - LOG_LEVEL: "debug" | "info" | "warn" | "error" (default: info)
func Init() *slog.Logger {
	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	level := parseLevel(os.Getenv("LOG_LEVEL"))

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.LevelKey:
				a.Key = "severity"
				var level slog.Level

				switch a.Value.Kind() {
				case slog.KindInt64:
					level = slog.Level(a.Value.Int64())
				case slog.KindAny:
					switch v := a.Value.Any().(type) {
					case slog.Level:
						level = v
					case slog.Leveler:
						level = v.Level()
					case int64:
						level = slog.Level(v)
					default:
						level = slog.LevelInfo
					}
				default:
					level = slog.LevelInfo
				}
				a.Value = slog.StringValue(levelToSeverity(level))
			case slog.MessageKey:
				a.Key = "message"
			}
			return a
		},
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler).With(
		slog.String("app", "homedocmanager"),
		slog.String("runtime", "cloud-run"),
	)
	slog.SetDefault(logger)

	// Route the standard library logger through slog so existing log.Printf calls
	// are emitted as structured logs (message only).
	log.SetFlags(0)
	log.SetOutput(&slogWriter{logger: logger, level: slog.LevelInfo})

	return logger
}

type slogWriter struct {
	logger *slog.Logger
	level  slog.Level
}

func (w *slogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}
	w.logger.Log(context.Background(), w.level, msg)
	return len(p), nil
}

func parseLevel(v string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func levelToSeverity(l slog.Level) string {
	switch {
	case l <= slog.LevelDebug:
		return "DEBUG"
	case l < slog.LevelWarn:
		return "INFO"
	case l < slog.LevelError:
		return "WARNING"
	default:
		return "ERROR"
	}
}
