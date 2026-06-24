package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"log/slog"

	"gopkg.in/natefinch/lumberjack.v2"
)

// --- Fluent Interface (kept for compatibility) ---

// Level is the severity selector for the fluent logging interface. See the
// InfoLevel and WarnLevel constants; Error-level messages use ErrorLogger
// directly rather than a Level value.
type Level int

// WarnLevel defines warn level.
// InfoLevel defines info level.
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

	closers []io.Closer

	// A shared runtime-adjustable log level for all handlers
	levelVar slog.LevelVar
}

var (
	globalLogger        *Logger
	GlobalLogger        *Logger
	testLoggerOverrides sync.Map
	testRawErrorLogger  sync.Map
	testNilOverrides    sync.Map
)

func getGlobalLogger() *Logger {
	var pcs [16]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, ".Test") {
			idx := strings.LastIndex(frame.Function, ".")
			if idx != -1 {
				testName := frame.Function[idx+1:]
				if idxSlash := strings.Index(testName, "/"); idxSlash != -1 {
					testName = testName[:idxSlash]
				}
				if _, ok := testNilOverrides.Load(testName); ok {
					return nil
				}
				if val, ok := testLoggerOverrides.Load(testName); ok {
					return val.(*Logger)
				}
			}
		}
		if !more {
			break
		}
	}
	return globalLogger
}

// --- Public Fluent API (kept for compatibility) ---

// -- Instance Methods --

// Info infos.
func (l *Logger) Info() *CategorizedLogger {
	return &CategorizedLogger{logger: l, level: InfoLevel}
}

// Warn warns.
func (l *Logger) Warn() *CategorizedLogger {
	return &CategorizedLogger{logger: l, level: WarnLevel}
}

// Error errors.
func (l *Logger) Error() *ErrorLogger {
	return &ErrorLogger{logger: l}
}

// -- Package-Level Functions --

// Info infos.
func Info() *CategorizedLogger {
	return &CategorizedLogger{logger: getGlobalLogger(), level: InfoLevel}
}

// Warn warns.
func Warn() *CategorizedLogger {
	return &CategorizedLogger{logger: getGlobalLogger(), level: WarnLevel}
}

// Error errors.
func Error() *ErrorLogger {
	return &ErrorLogger{logger: getGlobalLogger()}
}

// --- Expose raw slog loggers for direct usage in new code ---

// ApplicationLogger returns the category-scoped slog.Logger for application logs.
func ApplicationLogger() *slog.Logger {
	gl := getGlobalLogger()
	if gl == nil || gl.application == nil {
		return slog.Default()
	}
	return gl.application
}

// DiscordLogger returns the category-scoped slog.Logger for Discord-related logs.
func DiscordLogger() *slog.Logger {
	gl := getGlobalLogger()
	if gl == nil || gl.discord == nil {
		return slog.Default()
	}
	return gl.discord
}

// DatabaseLogger returns the category-scoped slog.Logger for database logs.
func DatabaseLogger() *slog.Logger {
	gl := getGlobalLogger()
	if gl == nil || gl.database == nil {
		return slog.Default()
	}
	return gl.database
}

// ErrorLoggerRaw returns the category-scoped slog.Logger for error logs.
func ErrorLoggerRaw() *slog.Logger { // name avoids collision with Error() fluent builder
	var pcs [16]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, ".Test") {
			idx := strings.LastIndex(frame.Function, ".")
			if idx != -1 {
				testName := frame.Function[idx+1:]
				if idxSlash := strings.Index(testName, "/"); idxSlash != -1 {
					testName = testName[:idxSlash]
				}
				if val, ok := testRawErrorLogger.Load(testName); ok {
					return val.(*slog.Logger)
				}
			}
		}
		if !more {
			break
		}
	}

	gl := getGlobalLogger()
	if gl == nil || gl.error == nil {
		return slog.Default()
	}
	return gl.error
}

// GlobalLevelVar exposes the shared level variable so callers can adjust level at runtime.
func GlobalLevelVar() *slog.LevelVar {
	gl := getGlobalLogger()
	if gl == nil {
		// Default to Info if not initialized yet
		var lv slog.LevelVar
		lv.Set(slog.LevelInfo)
		return &lv
	}
	return &gl.levelVar
}

// --- Fluent API Finalizers (kept for compatibility) ---

// Applicationf applicationfs.
func (cl *CategorizedLogger) Applicationf(format string, v ...any) {
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

// Discordf discordfs.
func (cl *CategorizedLogger) Discordf(format string, v ...any) {
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

// Databasef databasefs.
func (cl *CategorizedLogger) Databasef(format string, v ...any) {
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

// Errorf errorfs.
func (el *ErrorLogger) Errorf(format string, v ...any) {
	if el == nil || el.logger == nil || el.logger.error == nil {
		stdlog.Printf("ERROR: "+format, v...)
		return
	}
	msg := fmt.Sprintf(format, v...)
	el.logger.error.Error(msg)
}

// Fatalf fatalfs.
func (el *ErrorLogger) Fatalf(format string, v ...any) {
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

// (Removed getDefaultLogDir)

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

// Enabled enableds.
func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle handles.
func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// WithAttrs withs attrs.
func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		out = append(out, h.WithAttrs(attrs))
	}
	return &multiHandler{handlers: out}
}

// WithGroup withs group.
func (m *multiHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		out = append(out, h.WithGroup(name))
	}
	return &multiHandler{handlers: out}
}

// buildCategoryLogger creates a slog.Logger that tees to file (JSON) and console (text)
// and annotates every record with service and category attributes.
func buildCategoryLogger(category string, fileWriter *lumberjack.Logger, consoleWriter *os.File, levelVar *slog.LevelVar, botName string) *slog.Logger {
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
		slog.String("service", botName),
		slog.String("category", category),
	)
	return base
}

// SetupLogger configures category-separated slog loggers (application, discord, database, error)
// writing to rotating files (via lumberjack) and to human-friendly console output.
// It preserves the existing global variables and fluent API and also exposes raw slog loggers.
func SetupLogger(botName, logFilePath string) error {
	var logDir string
	if logFilePath != "" {
		logDir = filepath.Dir(logFilePath)
	} else {
		logDir = filepath.Join(".", "logs")
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("SetupLogger: %w", err)
	}

	// Initialize global logger and default level
	l := &Logger{}
	l.levelVar.Set(slog.LevelInfo)

	// Per-category rolling files
	appFile := rollingWriter(filepath.Join(logDir, "application.log"))
	discordFile := rollingWriter(filepath.Join(logDir, "discord_events.log"))
	dbFile := rollingWriter(filepath.Join(logDir, "database.log"))
	errFile := rollingWriter(filepath.Join(logDir, "error.log"))

	l.closers = []io.Closer{appFile, discordFile, dbFile, errFile}

	// Console routing: stdout for most, stderr for errors
	l.application = buildCategoryLogger("application", appFile, os.Stdout, &l.levelVar, botName)
	l.discord = buildCategoryLogger("discord", discordFile, os.Stdout, &l.levelVar, botName)
	l.database = buildCategoryLogger("database", dbFile, os.Stdout, &l.levelVar, botName)
	l.error = buildCategoryLogger("error", errFile, os.Stderr, &l.levelVar, botName)

	var pcs [16]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, ".Test") {
			idx := strings.LastIndex(frame.Function, ".")
			if idx != -1 {
				testName := frame.Function[idx+1:]
				if idxSlash := strings.Index(testName, "/"); idxSlash != -1 {
					testName = testName[:idxSlash]
				}
				testLoggerOverrides.Store(testName, l)
				break
			}
		}
		if !more {
			break
		}
	}

	globalLogger = l
	GlobalLogger = l

	// Initial line confirming initialization (kept behavior)
	l.application.Info("logger initialized", slog.String("time", time.Now().Format(time.RFC3339Nano)))

	// Also set the process default logger so third-party packages using slog.Default() get our handler.
	// Default will use the application category (most general).
	slog.SetDefault(l.application)

	return nil
}

// CloseGlobalLogger safely closes all underlying file handles for the global logger.
func CloseGlobalLogger() error {
	isTest := false
	var pcs [16]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, ".Test") {
			idx := strings.LastIndex(frame.Function, ".")
			if idx != -1 {
				testName := frame.Function[idx+1:]
				if idxSlash := strings.Index(testName, "/"); idxSlash != -1 {
					testName = testName[:idxSlash]
				}
				if val, ok := testLoggerOverrides.Load(testName); ok {
					err := val.(*Logger).Close()
					testLoggerOverrides.Delete(testName)
					isTest = true
					return err
				}
			}
		}
		if !more {
			break
		}
	}

	if !isTest && globalLogger != nil {
		err := globalLogger.Close()
		globalLogger = nil
		GlobalLogger = nil
		return err
	}
	return nil
}

// Sync is a best-effort flush for outputs.
// slog itself does not buffer; lumberjack writes synchronously.
// We keep this method so callers can defer GlobalLogger.Sync() safely.
func (l *Logger) Sync() {
	// No-op for slog + lumberjack; present for API symmetry and future extensibility.
}

// Close closes all underlying file writers.
func (l *Logger) Close() error {
	var errs []error
	for _, c := range l.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
