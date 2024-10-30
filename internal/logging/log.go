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

var levelNames = map[LogLevel]string{
	LevelPanic: "PANIC",
	LevelFatal: "FATAL",
	LevelError: "ERROR",
	LevelWarn:  "WARN",
	LevelInfo:  "INFO",
	LevelDebug: "DEBUG",
}
var levelNamesFormatted []string

var logLevel = DefaultLevel

var defaultLogger = log.New(os.Stdout, "", loggerFlags)
var errorLogger = log.New(os.Stderr, "", loggerFlags)

func levelNameSlice() []string {
	names := make([]string, 0, len(levelNames))
	for _, key := range slices.Sorted(maps.Keys(levelNames)) {
		name := levelNames[key]
		paddingLength := 6 - len(name)
		names = append(names, fmt.Sprintf("[%s]%-*s", name, paddingLength, " "))
	}
	return names
}

func init() {
	levelNamesFormatted = levelNameSlice()

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
	return levelNamesFormatted[level]
}

func SetLogLevelFromEnvironment(envVar string) error {
	value := os.Getenv(envVar)
	if value == "" {
		return nil
	}
	return SetLogLevelByName(value)
}
func SetLogLevelByName(levelName string) error {
	nameLookupMap := make(map[string]LogLevel)
	for k, v := range levelNames {
		nameLookupMap[v] = k
	}

	level, found := nameLookupMap[strings.ToUpper(levelName)]
	if !found {
		return fmt.Errorf("unknown log level: %s", levelName)
	}
	return SetLogLevel(level)
}

func SetLogLevel(level LogLevel) error {
	_, found := levelNames[level]
	if !found {
		return fmt.Errorf("unknown log level: %d", level)
	}
	logLevel = level
	return nil
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
