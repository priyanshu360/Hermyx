package logger

import (
	"hermyx/pkg/models"
	"io"
	"log"
	"os"
	"path/filepath"
)

type Logger struct {
	info         *log.Logger
	warn         *log.Logger
	debug        *log.Logger
	error        *log.Logger
	file         *os.File
	debugEnabled bool
}

func NewLogger(cfg *models.LogConfig) (*Logger, error) {
	var writers []io.Writer

	if cfg.ToStdout {
		writers = append(writers, os.Stdout)
	}

	var file *os.File
	if cfg.ToFile {
		if cfg.FilePath == "" {
			cfg.FilePath = "hermyx.log"
		}
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}

		f, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		file = f
		writers = append(writers, f)
	}

	multiWriter := io.MultiWriter(writers...)

	return &Logger{
		info:         log.New(multiWriter, cfg.Prefix+"INFO: ", cfg.Flags),
		warn:         log.New(multiWriter, cfg.Prefix+"WARN: ", cfg.Flags),
		debug:        log.New(multiWriter, cfg.Prefix+"DEBUG: ", cfg.Flags),
		error:        log.New(multiWriter, cfg.Prefix+"ERROR: ", cfg.Flags),
		file:         file,
		debugEnabled: true, // optional bool in your LogConfig
	}, nil
}

func (l *Logger) Info(msg string) {
	l.info.Println(msg)
}

func (l *Logger) Warn(msg string) {
	l.warn.Println(msg)
}

func (l *Logger) Debug(msg string) {
	if l.debugEnabled {
		l.debug.Println(msg)
	}
}

func (l *Logger) Error(msg string) {
	l.error.Println(msg)
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
