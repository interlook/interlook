package logger

import (
	"io"
	"os"
	"runtime"
	"strconv"

	"github.com/sirupsen/logrus"
)

var defaultLogger Logger

// Log fields
const (
	ProviderName = "providerName"
	ServiceName  = "serviceName"
)

// Logger is the default app logger
type Logger interface {
	logrus.FieldLogger
}

func init() {
	// FIXME: properly configure logger based on config ipalloc.
	// TODO: Add timestamp
	// TODO: Add package/ipalloc field to std logger?
	defaultLogger = logrus.StandardLogger()
	logrus.SetOutput(os.Stdout)
	//logrus.SetReportCaller(true)

	logrus.SetFormatter(&logrus.TextFormatter{})
	logLevel, _ := logrus.ParseLevel("DEBUG")
	logrus.SetLevel(logLevel)
	//logrus.Info("logger initialized")
}

// SetOutput sets the standard logger output.
func SetOutput(out io.Writer) {
	logrus.SetOutput(out)
}

// SetFormatter sets the standard logger formatter.
func SetFormatter(formatter logrus.Formatter) {
	logrus.SetFormatter(formatter)
}

// SetLevel sets the standard logger level.
func SetLevel(level logrus.Level) {
	logrus.SetLevel(level)
}

// GetLevel returns the standard logger level.
func GetLevel() logrus.Level {
	return logrus.GetLevel()
}

// Str adds a string field
func Str(key, value string) func(logrus.Fields) {
	return func(fields logrus.Fields) {
		fields[key] = value
	}
}

// DefaultLogger Gets the main logger
func DefaultLogger() Logger {
	return defaultLogger
}

func errInfo() (info string) {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame.Function + ":" + strconv.Itoa(frame.Line)
}
