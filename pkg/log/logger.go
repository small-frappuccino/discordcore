package log

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/alice-bnuy/discordcore/pkg/util"
)

type Category int

const (
	Application Category = iota
	DiscordEvents
	Database
)

type Logger struct {
	application *log.Logger
	discord     *log.Logger
	database    *log.Logger
	error       *log.Logger
}

var globalLogger *Logger

func SetupLogger() error {
	logDir := filepath.Dir(util.GetLogFilePath())

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

	globalLogger = &Logger{
		application: log.New(io.MultiWriter(os.Stdout, appLog), "", log.LstdFlags|log.Lmicroseconds),
		discord:     log.New(io.MultiWriter(os.Stdout, discordLog), "", log.LstdFlags|log.Lmicroseconds),
		database:    log.New(io.MultiWriter(os.Stdout, dbLog), "", log.LstdFlags|log.Lmicroseconds),
		error:       log.New(io.MultiWriter(os.Stderr, errorLog), "", log.LstdFlags|log.Lmicroseconds),
	}

	return nil
}

func Info(category Category, message string) {
	switch category {
	case Application:
		globalLogger.application.Println(message)
	case DiscordEvents:
		globalLogger.discord.Println(message)
	case Database:
		globalLogger.database.Println(message)
	}
}

func Infof(category Category, format string, v ...interface{}) {
	switch category {
	case Application:
		globalLogger.application.Printf(format, v...)
	case DiscordEvents:
		globalLogger.discord.Printf(format, v...)
	case Database:
		globalLogger.database.Printf(format, v...)
	}
}

func Error(message string) {
	globalLogger.error.Println(message)
}

func Errorf(format string, v ...interface{}) {
	globalLogger.error.Printf(format, v...)
}
