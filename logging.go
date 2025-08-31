package discordcore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the log level
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// Error messages
const (
	ErrCreateLogsFileMsg = "error creating log file: %w"
	ErrCreateLogsDirMsg  = "error creating logs directory: %w"
)

const LogsDirPath = "logs"

// String converts LogLevel to string
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Caller    string                 `json:"caller,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Logger is a structured and efficient logger
type Logger struct {
	level      LogLevel
	fileWriter io.Writer
	consoleOut io.Writer
	consoleErr io.Writer
	logFile    *os.File
	fields     map[string]interface{}
}

// LoggerConfig logger configuration
type LoggerConfig struct {
	Level          LogLevel
	LogDir         string
	EnableConsole  bool
	EnableFile     bool
	EnableJSON     bool
	IncludeCaller  bool
	FileNameFormat string // "2006-01-02" para daily rotation
}

// NewLogger creates a new logger instance
func NewLogger(config LoggerConfig) (*Logger, error) {
	logger := &Logger{
		level:      config.Level,
		consoleOut: os.Stdout,
		consoleErr: os.Stderr,
		fields:     make(map[string]interface{}),
	}

	// Configure log file if enabled
	if config.EnableFile {
		if err := os.MkdirAll(config.LogDir, 0755); err != nil {
			return nil, fmt.Errorf(ErrCreateLogsDirMsg, err)
		}

		filename := config.FileNameFormat
		if filename == "" {
			filename = "2006-01-02"
		}

		timestamp := time.Now().Format(filename)
		logPath := filepath.Join(config.LogDir, fmt.Sprintf("alicemains_%s.log", timestamp))

		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf(ErrCreateLogsFileMsg, err)
		}

		logger.logFile = logFile
		logger.fileWriter = logFile
	}

	return logger, nil
}

// WithField adds a field to the logger (returns new instance)
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := *l
	newLogger.fields = make(map[string]interface{})
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return &newLogger
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := *l
	newLogger.fields = make(map[string]interface{})
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return &newLogger
}

// WithContext adds context fields if available
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Here values can be extracted from context like request ID, user ID, etc.
	newLogger := *l
	newLogger.fields = make(map[string]interface{})
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Example of extracting values from context
	if userID := ctx.Value("userID"); userID != nil {
		newLogger.fields["userID"] = userID
	}
	if guildID := ctx.Value("guildID"); guildID != nil {
		newLogger.fields["guildID"] = guildID
	}
	if requestID := ctx.Value("requestID"); requestID != nil {
		newLogger.fields["requestID"] = requestID
	}

	return &newLogger
}

// log is the internal method that performs logging
func (l *Logger) log(level LogLevel, message string, err error) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level.String(),
		Message:   message,
		Fields:    l.fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	// Include caller if configured
	if l.shouldIncludeCaller() {
		if pc, file, line, ok := runtime.Caller(3); ok {
			funcName := runtime.FuncForPC(pc).Name()
			// Extract only the function name without the full path
			if idx := strings.LastIndex(funcName, "."); idx != -1 {
				funcName = funcName[idx+1:]
			}
			entry.Caller = fmt.Sprintf("%s:%d:%s", filepath.Base(file), line, funcName)
		}
	}

	// Write to file (structured JSON)
	if l.fileWriter != nil {
		jsonData, _ := json.Marshal(entry)
		fmt.Fprintln(l.fileWriter, string(jsonData))
	}

	// Write to console (human-readable format)
	if l.consoleOut != nil || l.consoleErr != nil {
		l.writeToConsole(entry)
	}
}

func (l *Logger) shouldIncludeCaller() bool {
	// For now, include caller only for ERROR and FATAL
	return true
}

func (l *Logger) writeToConsole(entry LogEntry) {
	var output io.Writer = l.consoleOut
	var icon string

	switch entry.Level {
	case "DEBUG":
		icon = "🔍"
	case "INFO":
		icon = "ℹ️"
	case "WARN":
		icon = "⚠️"
		output = l.consoleErr
	case "ERROR":
		icon = "❌"
		output = l.consoleErr
	case "FATAL":
		icon = "💀"
		output = l.consoleErr
	default:
		icon = "📝"
	}

	timestamp := entry.Timestamp[11:19] // Only HH:MM:SS

	message := fmt.Sprintf("%s [%s] %s %s", timestamp, entry.Level, icon, entry.Message)

	// Add fields if exist
	if len(entry.Fields) > 0 {
		var fieldParts []string
		for k, v := range entry.Fields {
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%v", k, v))
		}
		message += fmt.Sprintf(" | %s", strings.Join(fieldParts, " "))
	}

	// Add error if exists
	if entry.Error != "" {
		message += fmt.Sprintf(" | error=%s", entry.Error)
	}

	// Add caller if exists
	if entry.Caller != "" {
		message += fmt.Sprintf(" | %s", entry.Caller)
	}

	fmt.Fprintln(output, message)
}

// Public log methods
func (l *Logger) Debug(message string) {
	l.log(DebugLevel, message, nil)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	if DebugLevel >= l.level {
		l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
	}
}

func (l *Logger) Info(message string) {
	l.log(InfoLevel, message, nil)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	if InfoLevel >= l.level {
		l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
	}
}

func (l *Logger) Warn(message string) {
	l.log(WarnLevel, message, nil)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	if WarnLevel >= l.level {
		l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
	}
}

func (l *Logger) Error(message string) {
	l.log(ErrorLevel, message, nil)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	if ErrorLevel >= l.level {
		l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
	}
}

func (l *Logger) ErrorWithErr(message string, err error) {
	l.log(ErrorLevel, message, err)
}

func (l *Logger) Fatal(message string) {
	l.log(FatalLevel, message, nil)
	os.Exit(1)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FatalLevel, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

func (l *Logger) FatalWithErr(message string, err error) {
	l.log(FatalLevel, message, err)
	os.Exit(1)
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// Global logger for convenience
var GlobalLogger *Logger

// InitializeGlobalLogger initializes the global logger
func InitializeGlobalLogger(config LoggerConfig) error {
	logger, err := NewLogger(config)
	if err != nil {
		return err
	}
	GlobalLogger = logger
	return nil
}

// CloseGlobalLogger closes the global logger
func CloseGlobalLogger() error {
	if GlobalLogger != nil {
		return GlobalLogger.Close()
	}
	return nil
}

// Convenience functions for the global logger
func Debug(message string) {
	if GlobalLogger != nil {
		GlobalLogger.Debug(message)
	}
}

func Debugf(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Debugf(format, args...)
	}
}

func Info(message string) {
	if GlobalLogger != nil {
		GlobalLogger.Info(message)
	}
}

func Infof(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Infof(format, args...)
	}
}

func Warn(message string) {
	if GlobalLogger != nil {
		GlobalLogger.Warn(message)
	}
}

func Warnf(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Warnf(format, args...)
	}
}

func Error(message string) {
	if GlobalLogger != nil {
		GlobalLogger.Error(message)
	}
}

func Errorf(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Errorf(format, args...)
	}
}

func ErrorWithErr(message string, err error) {
	if GlobalLogger != nil {
		GlobalLogger.ErrorWithErr(message, err)
	}
}

func Fatal(message string) {
	if GlobalLogger != nil {
		GlobalLogger.Fatal(message)
	}
}

func Fatalf(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Fatalf(format, args...)
	}
}

func FatalWithErr(message string, err error) {
	if GlobalLogger != nil {
		GlobalLogger.FatalWithErr(message, err)
	}
}

// WithField creates a logger with an additional field
func WithField(key string, value interface{}) *Logger {
	if GlobalLogger != nil {
		return GlobalLogger.WithField(key, value)
	}
	return nil
}

// WithFields creates a logger with additional fields
func WithFields(fields map[string]interface{}) *Logger {
	if GlobalLogger != nil {
		return GlobalLogger.WithFields(fields)
	}
	return nil
}

// WithContext creates a logger with context
func WithContext(ctx context.Context) *Logger {
	if GlobalLogger != nil {
		return GlobalLogger.WithContext(ctx)
	}
	return nil
}

// ParseLogLevel converts string to LogLevel
func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DebugLevel
	case "INFO":
		return InfoLevel
	case "WARN", "WARNING":
		return WarnLevel
	case "ERROR":
		return ErrorLevel
	case "FATAL":
		return FatalLevel
	default:
		return InfoLevel
	}
}

func SetupLogger() error {
	logLevel := ParseLogLevel(os.Getenv("LOG_LEVEL"))
	loggerConfig := LoggerConfig{
		Level:          logLevel,
		LogDir:         LogsDirPath,
		EnableConsole:  true,
		EnableFile:     true,
		EnableJSON:     false,
		IncludeCaller:  true,
		FileNameFormat: "2006-01-02",
	}
	if err := InitializeGlobalLogger(loggerConfig); err != nil {
		return err
	}
	return nil
}
