package log

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
)

var logFile string
var logLevel string

func TestMain(m *testing.M) {
	rc := m.Run()
	cleanUP()
	os.Exit(rc)
}

func cleanUP() {
	_ = os.Remove("./interlook.log")
}

func init() {
	fmt.Println("starting log test")
	logLevel = "DEBUG"
	logFile = "interlook.log"
	if err := os.Remove(logFile); err != nil {
		fmt.Println("could not delete log file")
	}

	Init(logFile, logLevel)
	Debug("Init log testing")
	//readLogs()
}

func existInTxtLog(msg, function string) bool {
	file, _ := os.Open(logFile)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "msg=\""+msg) && strings.Contains(line, "function="+function) {
			return true
		}
	}
	return false
}

func TestDebug(t *testing.T) {
	//moreInfo := retrieveCallInfo()
	str := "test Debug"
	Debug(str)
	if !existInTxtLog(str, "TestDebug") {
		t.Error("test debug: logged msg not found")
	}
}

func TestDebugf(t *testing.T) {
	level := "debugf"
	str := "test " + level
	Debugf(str)
	if !existInTxtLog(str, "TestDebugf") {
		t.Error("test debugf: logged msg not found")
	}

}

func TestInfo(t *testing.T) {
	level := "info"
	str := "test " + level
	Info(str)
	if !existInTxtLog(str, "TestInfo") {
		t.Error("test info: logged msg not found")
	}

}

func TestInfof(t *testing.T) {
	level := "info"
	str := "test " + level
	Infof(str, level)
	if !existInTxtLog(str, "TestInfof") {
		t.Error("test TestInfof: logged msg not found")
	}

}

func TestWarn(t *testing.T) {
	level := "warn"
	str := "test " + level
	Warn(str)
	if !existInTxtLog(str, "TestWarn") {
		t.Error("test warn: logged msg not found")
	}

}

func TestWarnf(t *testing.T) {
	level := "warn"
	str := "test " + level
	Warnf(str, level)
	if !existInTxtLog(str, "TestWarnf") {
		t.Error("test warnf: logged msg not found")
	}

}

func TestError(t *testing.T) {
	level := "error"
	str := "test " + level
	Error(str)
	if !existInTxtLog(str, "TestError") {
		t.Error("test error: logged msg not found")
	}

}

func TestErrorf(t *testing.T) {
	//moreInfo := retrieveCallInfo()
	level := "error"
	str := "test " + level
	Errorf(str, level)
	//time.Sleep(300 * time.Millisecond)
	if !existInTxtLog(str, "TestErrorf") {
		t.Error("test errorf: logged msg not found")
	}

}
