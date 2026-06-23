package log

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerSetupAndClose(t *testing.T) {
	// Create a temp directory for logs
	tmpDir, err := os.MkdirTemp("", "discordcore-logs-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "test.log")

	// 1. Test setup
	err = SetupLogger("test-bot", logFilePath)
	if err != nil {
		t.Fatalf("SetupLogger failed: %v", err)
	}

	// Verify globalLogger and GlobalLogger are set
	if globalLogger == nil || GlobalLogger == nil {
		t.Errorf("globalLogger was not initialized")
	}

	// 2. Test direct slog loggers are not nil
	if ApplicationLogger() == nil {
		t.Errorf("ApplicationLogger should not be nil")
	}
	if DiscordLogger() == nil {
		t.Errorf("DiscordLogger should not be nil")
	}
	if DatabaseLogger() == nil {
		t.Errorf("DatabaseLogger should not be nil")
	}
	if ErrorLoggerRaw() == nil {
		t.Errorf("ErrorLoggerRaw should not be nil")
	}
	if GlobalLevelVar() == nil {
		t.Errorf("GlobalLevelVar should not be nil")
	}

	// 3. Test logging functions
	Info().Applicationf("app info message %d", 1)
	Warn().Applicationf("app warn message %d", 2)
	Info().Discordf("discord info message %d", 3)
	Warn().Discordf("discord warn message %d", 4)
	Info().Databasef("db info message %d", 5)
	Warn().Databasef("db warn message %d", 6)
	Error().Errorf("error message %d", 7)

	// Exercise other levels in CategorizedLogger switch default
	cl := &CategorizedLogger{logger: globalLogger, level: Level(-1)}
	cl.Applicationf("app default message")
	cl.Discordf("discord default message")
	cl.Databasef("db default message")

	// Exercise multiHandler with attributes and groups
	appLogger := ApplicationLogger().With(slog.String("key", "val"))
	appLogger.Info("message with attributes")
	groupLogger := appLogger.WithGroup("test-group")
	groupLogger.Info("message in group")

	// Verify we can check Enabled
	if !groupLogger.Enabled(context.Background(), slog.LevelInfo) {
		t.Logf("Logger not enabled for Info, which is unexpected with defaults")
	}

	// 4. Test logger sync and close
	GlobalLogger.Sync()
	err = CloseGlobalLogger()
	if err != nil {
		t.Errorf("CloseGlobalLogger failed: %v", err)
	}

	// 5. Test close when already closed
	err = CloseGlobalLogger()
	if err != nil {
		t.Errorf("CloseGlobalLogger on nil logger failed: %v", err)
	}
}

func TestNilGlobalLoggerFallbacks(t *testing.T) {
	// Temporarily clear global logger
	oldGlobal := globalLogger
	oldGlobalExport := GlobalLogger
	globalLogger = nil
	GlobalLogger = nil
	defer func() {
		globalLogger = oldGlobal
		GlobalLogger = oldGlobalExport
	}()

	// Verify fallback return values
	if ApplicationLogger() != slog.Default() {
		t.Errorf("expected fallback to slog.Default()")
	}
	if DiscordLogger() != slog.Default() {
		t.Errorf("expected fallback to slog.Default()")
	}
	if DatabaseLogger() != slog.Default() {
		t.Errorf("expected fallback to slog.Default()")
	}
	if ErrorLoggerRaw() != slog.Default() {
		t.Errorf("expected fallback to slog.Default()")
	}

	levelVar := GlobalLevelVar()
	if levelVar == nil || levelVar.Level() != slog.LevelInfo {
		t.Errorf("expected fallback levelVar to be Info")
	}

	// Verify fluent APIs handle nil loggers and print to stdlog
	var cl *CategorizedLogger
	cl.Applicationf("test nil cl app")
	cl.Discordf("test nil cl discord")
	cl.Databasef("test nil cl db")

	cl2 := &CategorizedLogger{logger: nil, level: InfoLevel}
	cl2.Applicationf("test cl2 app")
	cl2.Discordf("test cl2 discord")
	cl2.Databasef("test cl2 db")

	var el *ErrorLogger
	el.Errorf("test nil el error")

	el2 := &ErrorLogger{logger: nil}
	el2.Errorf("test el2 error")
}

func TestHelpers(t *testing.T) {
	reqID := GenerateRequestID()
	if len(reqID) != 32 {
		t.Errorf("expected 32-char hex request ID, got: %s", reqID)
	}

	// Test SetErrorLoggerRawForTest
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cleanup := SetErrorLoggerRawForTest(testLogger)
	if ErrorLoggerRaw() != testLogger {
		t.Errorf("failed to override error logger in test")
	}
	cleanup()
}

func TestEmitBlockingError(t *testing.T) {
	// Set up a custom logger to capture output
	var buf strings.Builder
	h := slog.NewTextHandler(&buf, nil)
	logger := slog.New(h)

	cleanup := SetErrorLoggerRawForTest(logger)
	defer cleanup()

	EmitBlockingError("blocking database error", errors.New("conn reset"), "req-12345")

	output := buf.String()
	if !strings.Contains(output, "blocking database error") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, "req-12345") {
		t.Errorf("expected output to contain request ID")
	}
	if !strings.Contains(output, "conn reset") {
		t.Errorf("expected output to contain error detail")
	}
	if !strings.Contains(output, "stack_trace") {
		t.Errorf("expected output to contain stack trace")
	}
}

func TestFatalf(t *testing.T) {
	if os.Getenv("BE_FATAL") == "1" {
		// Set up logger and trigger Fatalf
		tmpDir, err := os.MkdirTemp("", "discordcore-fatal-test")
		if err != nil {
			os.Exit(2)
		}
		defer os.RemoveAll(tmpDir)

		err = SetupLogger("test-bot-fatal", filepath.Join(tmpDir, "fatal.log"))
		if err != nil {
			os.Exit(3)
		}
		Error().Fatalf("simulated fatal error: %s", "abort")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatalf")
	cmd.Env = append(os.Environ(), "BE_FATAL=1")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ProcessState.ExitCode() != 1 {
			t.Errorf("expected exit status 1, got %v", exitErr.ProcessState.ExitCode())
		}
	} else {
		t.Errorf("expected exit error, got nil or other error: %v", err)
	}
}

func TestFatalfNilLogger(t *testing.T) {
	if os.Getenv("BE_FATAL_NIL") == "1" {
		// Make sure global logger is nil
		globalLogger = nil
		var el *ErrorLogger
		el.Fatalf("simulated nil fatal error")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatalfNilLogger")
	cmd.Env = append(os.Environ(), "BE_FATAL_NIL=1")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ProcessState.ExitCode() != 1 {
			t.Errorf("expected exit status 1, got %v", exitErr.ProcessState.ExitCode())
		}
	} else {
		t.Errorf("expected exit error, got nil or other error: %v", err)
	}
}
