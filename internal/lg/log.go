package lg

import (
    "context"
    "flag"
    "log"
    "os"
    "strings"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "time"
    "bytes"
)

// Field is a structured log field, aliasing zapcore.Field for flexibility.
type Field = zapcore.Field

func Any(key string, value any) Field         { return zap.Any(key, value) }
func String(key, value string) Field          { return zap.String(key, value) }
func Int(key string, value int) Field         { return zap.Int(key, value) }
func Bool(key string, value bool) Field       { return zap.Bool(key, value) }
func Float64(key string, value float64) Field { return zap.Float64(key, value) }
func Time(key string, value time.Time) Field  { return zap.Time(key, value) }

// Logger defines the minimal interface for structured logging.
type Logger interface {
    Info(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    With(fields ...Field) Logger
    Sync() error
    Debug(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
}

// Config holds logging configuration options.
type Config struct {
    ServiceName string 
    Debug       bool   
    Format      string // "json" or "console"
}

// NewConfigFromFlags parses standard flags: -debug and -log-format.
func NewConfigFromFlags(serviceName string) *Config {
    fs := flag.NewFlagSet(serviceName, flag.ExitOnError)
    debug := fs.Bool("debug", false, "enable debug logging")
    format := fs.String("log-format", "json", "json or console")
    fs.Parse(os.Args[1:])
    return &Config{ServiceName: serviceName, Debug: *debug, Format: *format}
}

// NewLogger builds a zap-based Logger based on cfg.
// It configures encoding, level, sampling, and initial fields.
func New(cfg *Config) Logger {
    var baseCfg zap.Config
    if cfg.Debug {
        baseCfg = zap.NewDevelopmentConfig()
        baseCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    } else {
        baseCfg = zap.NewProductionConfig()
    }

    // Allow console or JSON output
    baseCfg.Encoding = cfg.Format
    baseCfg.EncoderConfig.TimeKey = "timestamp"
    baseCfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
    baseCfg.InitialFields = map[string]any{"service": cfg.ServiceName}

    // Enable sampling for high-throughput logs
    baseCfg.Sampling = &zap.SamplingConfig{Initial: 100, Thereafter: 100}

    logger, err := baseCfg.Build(zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
    if err != nil {
        // Fall back to standard log if zap fails
        log.Printf("[FATAL] cannot initialize zap logger: %v", err)
        return defaultLogger{}
    }

    return &zapLogger{l: logger}
}

// zapLogger wraps a *zap.Logger to implement Logger.
type zapLogger struct{ l *zap.Logger }

func (z *zapLogger) Info(msg string, fields ...Field) {
    z.l.Info(msg, fields...)
}

func (z *zapLogger) Error(msg string, fields ...Field) {
    z.l.Error(msg, fields...)
}

func (z *zapLogger) With(fields ...Field) Logger {
    return &zapLogger{z.l.With(fields...)}
}

func (z *zapLogger) Sync() error {
    return z.l.Sync()
}

func (z *zapLogger) Debug(msg string, fields ...Field){}
func (z *zapLogger) Warn(msg string, fields ...Field){}


// defaultLogger falls back to the standard log package.
type defaultLogger struct{}

func (d defaultLogger) Info(msg string, fields ...Field) {
    log.Println("INFO:", msg, flatten(fields...))
}

func (d defaultLogger) Error(msg string, fields ...Field) {
    log.Println("ERROR:", msg, flatten(fields...))
}

func (d defaultLogger) With(fields ...Field) Logger { return d }
func (d defaultLogger) Sync() error { return nil }
func (d defaultLogger) Debug(msg string, fields ...Field){}
func (d defaultLogger) Warn(msg string, fields ...Field){}

// flattenFields converts a list of zap log fields into a space-separated string of key-value pairs.
// The output format is "key1=value1 key2=value2 ...", similar to logfmt but using zap's encoding rules.
//
// This is useful for:
//   - Converting structured zap logs into a flat string (e.g., for text output or metrics).
//   - Debugging field contents without relying on a full logger.
//   - Serializing fields in a human-readable format.
//
// Notes:
//   - Uses zap's built-in encoder to ensure consistent formatting (e.g., durations, errors, nested objects).
//   - Omits standard log metadata like timestamps and levels.
//   - Trims trailing whitespace for cleaner output.
//   - Returns an empty string if no fields are provided.
//
// Example:
//   fields := []Field{
//       zap.String("user", "alice"),
//       zap.Int("attempts", 3),
//   }
//   fmt.Println(flattenFields(fields...)) // "user=alice attempts=3"
func flatten(fields ...Field) string {
	buf := new(bytes.Buffer)
	enc := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		MessageKey:     "",
		LevelKey:       "",
		TimeKey:        "",
		NameKey:        "",
		CallerKey:      "",
		FunctionKey:    "",
		StacktraceKey:  "",
		LineEnding:     " ",
		EncodeTime:     nil,
		EncodeLevel:    nil,
		EncodeDuration: nil,
		EncodeCaller:   nil,
	})
	entry := zapcore.Entry{}
	buffer, _ := enc.EncodeEntry(entry, fields)
	defer buffer.Free()
	buf.Write(buffer.Bytes())
	return strings.TrimSpace(buf.String())
}

// context key type for carrying Logger
// unexported to avoid collisions
type ctxKey struct{}

// Attach returns a new context with the provided Logger.
func Attach(ctx context.Context, lg Logger) context.Context {
    return context.WithValue(ctx, ctxKey{}, lg)
}

// FromContext retrieves the Logger from ctx, or falls back to defaultLogger.
func FromContext(ctx context.Context) Logger {
    if lg, ok := ctx.Value(ctxKey{}).(Logger); ok && lg != nil {
        return lg
    }
    return defaultLogger{}
}

// noopLogger does absolutely nothing. For test only
type noopLogger struct{}
func (noopLogger) Info(msg string, _ ...Field) {}
func (noopLogger) Debug(msg string, _ ...Field) {}
func (noopLogger) Error(msg string, _ ...Field) {}
func (noopLogger) Warn(msg string, _ ...Field) {}
func (noopLogger) With(_ ...Field) Logger { return noopLogger{} }
func (noopLogger) Sync() error { return nil }
var Discard Logger = noopLogger{}