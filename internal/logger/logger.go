package logger

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	oidfed "github.com/go-oidfed/lib"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
)

func mustGetFile(path string) io.Writer {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		panic(err)
	}
	file, err := getFile(path)
	if err != nil {
		panic(err)
	}
	return file
}

func getFile(path string) (io.Writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	return f, errors.WithStack(err)
}

var accessLogger *exchangeableWriter

// MustGetAccessLogger open the server access logger; on failure the program exits
func MustGetAccessLogger() io.Writer {
	accessLogger = &exchangeableWriter{
		Writer: mustGetAccessLogger(),
	}
	return accessLogger
}

// MustUpdateAccessLogger updates the writer of the access logger
func MustUpdateAccessLogger() {
	accessLogger.SetOutput(mustGetAccessLogger())
}

// AccessLogger returns the shared access log writer. Both the main server and
// the admin API server should use this as the output for their fiber logger
// middleware so that MustUpdateAccessLogger swaps the underlying writer for
// both at once.
func AccessLogger() io.Writer {
	if accessLogger == nil {
		return io.Discard
	}
	return accessLogger
}

func mustGetAccessLogger() io.Writer {
	return mustGetLogWriter(config.Get().Logging.Access, "access.log")
}

type exchangeableWriter struct {
	io.Writer
}

// SetOutput updates the internal writer
func (w *exchangeableWriter) SetOutput(out io.Writer) {
	w.Writer = out
}

func mustGetLogWriter(logConf config.LoggerConf, logfileName string) io.Writer {
	var loggers []io.Writer
	if logConf.StdErr {
		loggers = append(loggers, os.Stderr)
	}
	if logDir := logConf.Dir; logDir != "" {
		loggers = append(loggers, mustGetFile(filepath.Join(logDir, logfileName)))
	}
	switch len(loggers) {
	case 0:
		return io.Discard
	case 1:
		return loggers[0]
	default:
		return io.MultiWriter(loggers...)
	}
}

func parseLogLevel() zerolog.Level {
	logLevel := strings.ToLower(config.Get().Logging.Internal.Level)
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		log.Error().Str("level", logLevel).Err(err).Msg("Unknown log level")
		return zerolog.InfoLevel
	}
	return level
}

// buildLogWriter wraps out with the appropriate zerolog writer for the given
// format. "json" (or empty) passes raw JSON through; any other value produces
// a zerolog.ConsoleWriter. noColor disables ANSI color codes (for file output).
func buildLogWriter(out io.Writer, format string, noColor bool) io.Writer {
	switch strings.ToLower(format) {
	case "json", "":
		return out
	default: // "console"
		return zerolog.ConsoleWriter{
			Out:        out,
			NoColor:    noColor,
			TimeFormat: zerolog.TimeFieldFormat,
		}
	}
}

// Init initializes the logger
func Init() {
	SetOutput()
	MustGetAccessLogger()
	if DebugEnabled() {
		oidfed.EnableDebugLogging()
	}
}

// SetOutput sets the logging output
func SetOutput() {
	logLevel := parseLogLevel()
	zerolog.SetGlobalLevel(logLevel)

	conf := config.Get().Logging.Internal

	var writers []io.Writer
	if conf.StdErr {
		writers = append(writers, buildLogWriter(os.Stderr, conf.StdErrFormat, false))
	}
	if conf.Dir != "" {
		file := mustGetFile(filepath.Join(conf.Dir, "lighthouse.log"))
		writers = append(writers, buildLogWriter(file, conf.DirFormat, true))
	}

	var writer io.Writer
	switch len(writers) {
	case 0:
		writer = io.Discard
	case 1:
		writer = writers[0]
	default:
		writer = io.MultiWriter(writers...)
	}

	ctx := log.With().Timestamp()
	if logLevel <= zerolog.DebugLevel {
		ctx = ctx.Caller()
	}
	log.Logger = ctx.Logger().Output(writer)

	// Wire the lib's logger to the same output and level.
	oidfed.SetLogOutput(writer)
	oidfed.SetLogLevel(logLevel)
}

func DebugEnabled() bool {
	return parseLogLevel() <= zerolog.DebugLevel
}
