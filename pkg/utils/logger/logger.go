package logger

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"hermyx/pkg/models"
	"io"
	"os"
	"path/filepath"
)

type AsyncWriter struct {
	ch   chan []byte
	done chan struct{}
}

func NewAsyncWriter(w io.Writer, bufferSize int) *AsyncWriter {
	aw := &AsyncWriter{
		ch:   make(chan []byte, bufferSize),
		done: make(chan struct{}),
	}
	go func() {
		for msg := range aw.ch {
			_, _ = w.Write(msg)
		}
		close(aw.done)
	}()
	return aw
}

func (a *AsyncWriter) Write(p []byte) (int, error) {
	cp := make([]byte, len(p))
	copy(cp, p)

	select {
	case a.ch <- cp:
	default:
		// buffer full–drop log
	}
	return len(p), nil
}

func (a *AsyncWriter) Close() {
	close(a.ch)
	<-a.done
}


type Logger struct {
	file        *os.File
	log         zerolog.Logger
	asyncWriter *AsyncWriter
}

func NewLogger(cfg *models.LogConfig) (*Logger, error) {
	var writers []io.Writer

	if cfg.ToStdout {
		writers = append(writers, zerolog.ConsoleWriter{Out: os.Stdout})
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

	multi := io.MultiWriter(writers...)
	async := NewAsyncWriter(multi, 10000)

	zlog := zerolog.New(async).With().Timestamp().Logger()

	if cfg.DebugEnabled {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Logger = zlog

	return &Logger{
		file:        file,
		log:         zlog,
		asyncWriter: async,
	}, nil
}

func (l *Logger) Info(msg string)  { l.log.Info().Msg(msg) }
func (l *Logger) Warn(msg string)  { l.log.Warn().Msg(msg) }
func (l *Logger) Debug(msg string) { l.log.Debug().Msg(msg) }
func (l *Logger) Error(msg string) { l.log.Error().Msg(msg) }

func (l *Logger) Close() error {
	if l.asyncWriter != nil {
		l.asyncWriter.Close()
	}
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
