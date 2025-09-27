package log

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"time"
)

// --- Fluent Interface ---

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

// Logger is a simple structured logger wrapper around several stdlib loggers.
type Logger struct {
	application *stdlog.Logger
	discord     *stdlog.Logger
	database    *stdlog.Logger
	error       *stdlog.Logger
}

var globalLogger *Logger

// GlobalLogger is a convenience alias used by some initialization helpers in the project.
var GlobalLogger *Logger

// --- Public Fluent API ---

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

// --- Fluent API Finalizers ---

func (cl *CategorizedLogger) Applicationf(format string, v ...interface{}) {
	cl.log(cl.logger.application, format, v...)
}

func (cl *CategorizedLogger) Discordf(format string, v ...interface{}) {
	cl.log(cl.logger.discord, format, v...)
}

func (cl *CategorizedLogger) Databasef(format string, v ...interface{}) {
	cl.log(cl.logger.database, format, v...)
}

func (cl *CategorizedLogger) log(target *stdlog.Logger, format string, v ...interface{}) {
	if cl.logger == nil {
		stdlog.Printf(format, v...)
		return
	}
	msg := format
	if cl.level == WarnLevel {
		msg = "WARN: " + msg
	}
	cl.logger.writeTo(target, msg, v...)
}

func (el *ErrorLogger) Errorf(format string, v ...interface{}) {
	if el.logger == nil {
		stdlog.Printf("ERROR: "+format, v...)
		return
	}
	el.logger.writeTo(el.logger.error, "ERROR: "+format, v...)
}

func (el *ErrorLogger) Fatalf(format string, v ...interface{}) {
	if el.logger == nil {
		stdlog.Fatalf("FATAL: "+format, v...)
	}
	el.logger.writeTo(el.logger.error, "FATAL: "+format, v...)
	os.Exit(1)
}

// --- Initialization & Helpers ---

func getDefaultLogDir() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "discordcore", "logs")
	}
	return filepath.Join(".", "logs")
}

func SetupLogger() error {
	logDir := getDefaultLogDir()
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}

	appLog, err := os.OpenFile(filepath.Join(logDir, "application.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}
	discordLog, err := os.OpenFile(filepath.Join(logDir, "discord_events.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}
	dbLog, err := os.OpenFile(filepath.Join(logDir, "database.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}
	errorLog, err := os.OpenFile(filepath.Join(logDir, "error.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}

	flags := stdlog.LstdFlags | stdlog.Lmicroseconds
	globalLogger = &Logger{
		application: stdlog.New(io.MultiWriter(os.Stdout, appLog), "APP ", flags),
		discord:     stdlog.New(io.MultiWriter(os.Stdout, discordLog), "DISCORD ", flags),
		database:    stdlog.New(io.MultiWriter(os.Stdout, dbLog), "DB ", flags),
		error:       stdlog.New(io.MultiWriter(os.Stderr, errorLog), "ERROR ", flags),
	}
	GlobalLogger = globalLogger
	globalLogger.Info().Applicationf("logger initialized at %s", time.Now().Format(time.RFC3339Nano))
	return nil
}

func (l *Logger) writeTo(target *stdlog.Logger, format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	if target == nil {
		stdlog.Printf("%s\n", message)
		return
	}
	target.Printf("%s", message)
}
