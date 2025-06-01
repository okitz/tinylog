package logger

import (
	"fmt"
	"os"
	"time"
)

type Logger struct {
	nodeID string
	out    *os.File
	level  LogLevel
}

type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

var levelLabels = map[LogLevel]string{
	DebugLevel: "DEBUG",
	InfoLevel:  "INFO ",
	WarnLevel:  "WARN ",
	ErrorLevel: "ERROR",
}

var levelColors = map[LogLevel]string{
	DebugLevel: "\033[34m", // blue
	InfoLevel:  "\033[32m", // green
	WarnLevel:  "\033[33m", // yellow
	ErrorLevel: "\033[31m", // red
}

const colorReset = "\033[0m"

func New(nodeID string, level LogLevel) *Logger {
	return &Logger{
		nodeID: nodeID,
		out:    os.Stdout,
		level:  level,
	}
}

func (l *Logger) log(level LogLevel, format string, args ...any) {
	if level < l.level {
		return
	}
	ts := time.Now().UnixMilli()
	msg := fmt.Sprintf(format, args...)
	label := levelLabels[level]
	color := levelColors[level]

	fmt.Fprintf(
		l.out,
		"%s[%s %d] %s%s %s\n",
		color, label, ts, l.nodeID, colorReset, msg,
	)
}

// 各レベルごとのショートカット
func (l *Logger) Debug(format string, args ...any) { l.log(DebugLevel, format, args...) }
func (l *Logger) Info(format string, args ...any)  { l.log(InfoLevel, format, args...) }
func (l *Logger) Warn(format string, args ...any)  { l.log(WarnLevel, format, args...) }

func (l *Logger) Error(err error, format string, args ...any) {
	format = format + ": " + err.Error()
	l.log(ErrorLevel, format, args...)
}
