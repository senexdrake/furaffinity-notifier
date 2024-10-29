package logging

import (
	"fmt"
	"log"
	"maps"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

type LogLevel int

const (
	LevelPanic LogLevel = iota
	LevelFatal
	LevelError
	LevelWarn
	LevelInfo
	LevelDebug
)

const DefaultLevel = LevelInfo
const DefaultCalldepth = 3
const stdErrThresholdLevel = LevelError
const loggerFlags = log.Ldate | log.Ltime | log.Lshortfile | log.Lmicroseconds

var levelNames []string

var logLevel = DefaultLevel

var defaultLogger = log.New(os.Stdout, "", loggerFlags)
var errorLogger = log.New(os.Stderr, "", loggerFlags)

func levelNameSlice() []string {
	levelNameMap := map[LogLevel]string{
		LevelPanic: "[PANIC] ",
		LevelFatal: "[FATAL] ",
		LevelError: "[ERROR] ",
		LevelWarn:  "[WARN]  ",
		LevelInfo:  "[INFO]  ",
		LevelDebug: "[DEBUG] ",
	}

	names := make([]string, 0, len(levelNameMap))
	for _, key := range slices.Sorted(maps.Keys(levelNameMap)) {
		names = append(names, levelNameMap[key])
	}
	return names
}

func init() {
	levelNames = levelNameSlice()

	log.SetOutput(os.Stdout)
	log.SetFlags(loggerFlags)
}

func callerInfo(calldepth int) string {
	_, file, no, _ := runtime.Caller(calldepth)

	fileParts := strings.Split(file, "/")
	caller := fileParts[len(fileParts)-2:]
	return strings.Join(caller, "/") + ":" + strconv.Itoa(no)
}

func loggerForLevel(level LogLevel) *log.Logger {
	if level <= stdErrThresholdLevel {
		return errorLogger
	}
	return defaultLogger
}

func levelName(level LogLevel) string {
	return levelNames[level]
}

func SetLogLevel(level LogLevel) {
	logLevel = level
}

func Logf(level LogLevel, calldepth int, msg string, args ...any) {
	if logLevel < level {
		return
	}
	if calldepth == 0 {
		calldepth = DefaultCalldepth
	}

	newArgs := []any{levelName(level)}

	err := loggerForLevel(level).Output(calldepth, fmt.Sprintf("\t%s"+msg, append(newArgs, args...)...))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: Could not write log message: %v", err)
	}
}

func Info(msg string) {
	Logf(LevelInfo, DefaultCalldepth, msg)
}

func Infof(msg string, args ...any) {
	Logf(LevelInfo, DefaultCalldepth, msg, args...)
}

func Warn(msg string) {
	Logf(LevelWarn, DefaultCalldepth, msg)
}

func Warnf(msg string, args ...any) {
	Logf(LevelWarn, DefaultCalldepth, msg, args...)
}

func Error(msg string) {
	Logf(LevelError, DefaultCalldepth, msg)
}

func Errorf(msg string, args ...any) {
	Logf(LevelError, DefaultCalldepth, msg, args...)
}

func Debug(msg string) {
	Logf(LevelDebug, DefaultCalldepth, msg)
}

func Debugf(msg string, args ...any) {
	Logf(LevelDebug, DefaultCalldepth, msg, args...)
}

func Panic(msg string) {
	Logf(LevelPanic, DefaultCalldepth, msg)
}

func Panicf(msg string, args ...any) {
	Logf(LevelPanic, DefaultCalldepth, msg, args...)
}

func Fatal(msg string) {
	Logf(LevelFatal, DefaultCalldepth, msg)
}

func Fatalf(msg string, args ...any) {
	Logf(LevelFatal, DefaultCalldepth, msg, args...)
}
