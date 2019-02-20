package logger

import (
	"os"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"
)

// Init initialize logger
// Don't use init() otherwise get called before the conf file is parsed
func Init() {

	log.SetFormatter(&log.TextFormatter{})
	logLevel, _ := log.ParseLevel("INFO")
	log.SetLevel(logLevel)
	log.SetOutput(os.Stdout)
	log.Info("logger initialized")
}

func errInfo() (info string) {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame.Function + ":" + strconv.Itoa(frame.Line)
}
