package log

import (
	"context"
	"fmt"
	stdlog "log"
	"os"
	"path/filepath"
	"time"

	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/util"
	"gopkg.in/natefinch/lumberjack.v2"
)

// --- Fluent Interface (kept for compatibility) ---

type Level int

const (
	InfoLevel Level = iota
	WarnLevel
)

// CategorizedLogger is an intermediate struct for building a log message
// for the Info and Warn levels.
type CategorizedLogger struct {
	logger *Logger
	level  Level
}

// ErrorLogger is an intermediate struct for building an Error level log message.
type ErrorLogger struct {
	logger *Logger
}

// --- Logger Struct & Setup ---

// Logger wraps slog loggers for different categories to preserve the existing API
// while enabling direct slog usage for new code.
type Logger struct {
	application *slog.Logger
	discord     *slog.Logger
	database    *slog.Logger
	error       *slog.Logger

	// A shared runtime-adjustable log level for all handlers
	levelVar slog.LevelVar
}

var globalLogger *Logger

// GlobalLogger is a convenience alias used by some initialization helpers in the project.
var GlobalLogger *Logger

// --- Public Fluent API (kept for compatibility) ---

// -- Instance Methods --

func (l *Logger) Info() *CategorizedLogger {
	return &CategorizedLogger{logger: l, level: InfoLevel}
}

func (l *Logger) Warn() *CategorizedLogger {
	return &CategorizedLogger{logger: l, level: WarnLevel}
}

func (l *Logger) Error() *ErrorLogger {
	return &ErrorLogger{logger: l}
}

// -- Package-Level Functions --

func Info() *CategorizedLogger {
	return &CategorizedLogger{logger: globalLogger, level: InfoLevel}
}

func Warn() *CategorizedLogger {
	return &CategorizedLogger{logger: globalLogger, level: WarnLevel}
}

func Error() *ErrorLogger {
	return &ErrorLogger{logger: globalLogger}
}

// --- Expose raw slog loggers for direct usage in new code ---

// ApplicationLogger returns the category-scoped slog.Logger for application logs.
func ApplicationLogger() *slog.Logger {
	if globalLogger == nil || globalLogger.application == nil {
		return slog.Default()
	}
	return globalLogger.application
}

// DiscordLogger returns the category-scoped slog.Logger for Discord-related logs.
func DiscordLogger() *slog.Logger {
	if globalLogger == nil || globalLogger.discord == nil {
		return slog.Default()
	}
	return globalLogger.discord
}

// DatabaseLogger returns the category-scoped slog.Logger for database logs.
func DatabaseLogger() *slog.Logger {
	if globalLogger == nil || globalLogger.database == nil {
		return slog.Default()
	}
	return globalLogger.database
}

// ErrorLoggerRaw returns the category-scoped slog.Logger for error logs.
func ErrorLoggerRaw() *slog.Logger { // name avoids collision with Error() fluent builder
	if globalLogger == nil || globalLogger.error == nil {
		return slog.Default()
	}
	return globalLogger.error
}

// GlobalLevelVar exposes the shared level variable so callers can adjust level at runtime.
func GlobalLevelVar() *slog.LevelVar {
	if globalLogger == nil {
		// Default to Info if not initialized yet
		var lv slog.LevelVar
		lv.Set(slog.LevelInfo)
		return &lv
	}
	return &globalLogger.levelVar
}

// --- Fluent API Finalizers (kept for compatibility) ---

func (cl *CategorizedLogger) Applicationf(format string, v ...interface{}) {
	if cl == nil || cl.logger == nil || cl.logger.application == nil {
		stdlog.Printf(format, v...)
		return
	}
	msg := fmt.Sprintf(format, v...)
	switch cl.level {
	case InfoLevel:
		cl.logger.application.Info(msg)
	case WarnLevel:
		cl.logger.application.Warn(msg)
	default:
		cl.logger.application.Info(msg)
	}
}

func (cl *CategorizedLogger) Discordf(format string, v ...interface{}) {
	if cl == nil || cl.logger == nil || cl.logger.discord == nil {
		stdlog.Printf(format, v...)
		return
	}
	msg := fmt.Sprintf(format, v...)
	switch cl.level {
	case InfoLevel:
		cl.logger.discord.Info(msg)
	case WarnLevel:
		cl.logger.discord.Warn(msg)
	default:
		cl.logger.discord.Info(msg)
	}
}

func (cl *CategorizedLogger) Databasef(format string, v ...interface{}) {
	if cl == nil || cl.logger == nil || cl.logger.database == nil {
		stdlog.Printf(format, v...)
		return
	}
	msg := fmt.Sprintf(format, v...)
	switch cl.level {
	case InfoLevel:
		cl.logger.database.Info(msg)
	case WarnLevel:
		cl.logger.database.Warn(msg)
	default:
		cl.logger.database.Info(msg)
	}
}

func (el *ErrorLogger) Errorf(format string, v ...interface{}) {
	if el == nil || el.logger == nil || el.logger.error == nil {
		stdlog.Printf("ERROR: "+format, v...)
		return
	}
	msg := fmt.Sprintf(format, v...)
	el.logger.error.Error(msg)
}

func (el *ErrorLogger) Fatalf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if el == nil || el.logger == nil || el.logger.error == nil {
		stdlog.Fatalf("FATAL: %s", msg)
		return
	}
	// slog has no Fatal level; log as Error and exit.
	el.logger.error.Error(msg, slog.String("fatal", "true"))
	os.Exit(1)
}

// --- Initialization & Helpers ---

func getDefaultLogDir() string {
	// Prefer the unified, OS-specific log location logic from util.
	// This keeps Linux/macOS/Windows consistent with the rest of the app's filesystem layout.
	if logPath := util.GetLogFilePath(); logPath != "" {
		return filepath.Dir(logPath)
	}
	return filepath.Join(".", "logs")
}

// rollingWriter creates a lumberjack-backed writer with sane defaults.
func rollingWriter(path string) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    50,   // megabytes per file
		MaxBackups: 3,    // number of old files to keep
		MaxAge:     30,   // days
		Compress:   true, // gzip old logs
	}
}

// multiHandler fans out records to multiple handlers (e.g., JSON file + console).
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		out = append(out, h.WithAttrs(attrs))
	}
	return &multiHandler{handlers: out}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		out = append(out, h.WithGroup(name))
	}
	return &multiHandler{handlers: out}
}

// buildCategoryLogger creates a slog.Logger that tees to file (JSON) and console (text)
// and annotates every record with service and category attributes.
func buildCategoryLogger(category string, fileWriter *lumberjack.Logger, consoleWriter *os.File, levelVar *slog.LevelVar) *slog.Logger {
	jsonHandler := slog.NewJSONHandler(fileWriter, &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: true,
	})
	textHandler := slog.NewTextHandler(consoleWriter, &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: true,
	})

	handler := &multiHandler{handlers: []slog.Handler{jsonHandler, textHandler}}
	base := slog.New(handler).With(
		slog.String("service", util.EffectiveBotName()),
		slog.String("category", category),
	)
	return base
}

// SetupLogger configures category-separated slog loggers (application, discord, database, error)
// writing to rotating files (via lumberjack) and to human-friendly console output.
// It preserves the existing global variables and fluent API and also exposes raw slog loggers.
func SetupLogger() error {
	logDir := getDefaultLogDir()
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}

	// Initialize global logger and default level
	l := &Logger{}
	l.levelVar.Set(slog.LevelInfo)

	// Per-category rolling files
	appFile := rollingWriter(filepath.Join(logDir, "application.log"))
	discordFile := rollingWriter(filepath.Join(logDir, "discord_events.log"))
	dbFile := rollingWriter(filepath.Join(logDir, "database.log"))
	errFile := rollingWriter(filepath.Join(logDir, "error.log"))

	// Console routing: stdout for most, stderr for errors
	l.application = buildCategoryLogger("application", appFile, os.Stdout, &l.levelVar)
	l.discord = buildCategoryLogger("discord", discordFile, os.Stdout, &l.levelVar)
	l.database = buildCategoryLogger("database", dbFile, os.Stdout, &l.levelVar)
	l.error = buildCategoryLogger("error", errFile, os.Stderr, &l.levelVar)

	globalLogger = l
	GlobalLogger = l

	// Initial line confirming initialization (kept behavior)
	l.application.Info("logger initialized", slog.String("time", time.Now().Format(time.RFC3339Nano)))

	// Also set the process default logger so third-party packages using slog.Default() get our handler.
	// Default will use the application category (most general).
	slog.SetDefault(l.application)

	return nil
}

// Sync is a best-effort flush for outputs.
// slog itself does not buffer; lumberjack writes synchronously.
// We keep this method so callers can defer GlobalLogger.Sync() safely.
func (l *Logger) Sync() {
	// No-op for slog + lumberjack; present for API symmetry and future extensibility.
}
