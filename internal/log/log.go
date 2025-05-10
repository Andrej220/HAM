
package log

import (
    "context"
    "flag"
    "log/slog"
    "os"
)

type Config struct {
    ServiceName string
    Debug       bool
    Format      string // "text" or "json"
}

// NewConfigFromFlags parses -debug and -log-format flags.
func NewConfigFromFlags(serviceName string) *Config {
    fs := flag.NewFlagSet(serviceName, flag.ExitOnError)
    debug := fs.Bool("debug", false, "enable debug logging")
    format := fs.String("log-format", "json", "text or json")
    fs.Parse(os.Args[1:])
    return &Config{ServiceName: serviceName, Debug: *debug, Format: *format}
}

// NewLogger builds a *slog.Logger per the given Config.
// Callers are responsible for holding the returned logger,
// or pushing it into context with Attach().
func NewLogger(cfg *Config) *slog.Logger {
    level := slog.LevelInfo
    if cfg.Debug {
        level = slog.LevelDebug
    }

    opts := &slog.HandlerOptions{ Level:     level, AddSource: true, }

    var handler slog.Handler
    if cfg.Format == "text" {
        handler = slog.NewTextHandler(os.Stdout, opts)
    } else {
        handler = slog.NewJSONHandler(os.Stdout, opts)
    }

    // include service name in every record
    return slog.New(handler).With("service", cfg.ServiceName)
}

// loggerCtxKey is an unexported type for context keys.
type loggerCtxKey struct{}

// Attach returns a child context that carries the given logger.
func Attach(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, logger)
}

// FromContext unwraps the logger from ctx, or returns a basic stderr logger
// if none was attached.
func FromContext(ctx context.Context) *slog.Logger {
    if lg := ctx.Value(loggerCtxKey{}); lg != nil {
        if l, ok := lg.(*slog.Logger); ok {
            return l
        }
    }
    // Fallback logger: writes to stderr at Info level
    handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
    return slog.New(handler)
}
