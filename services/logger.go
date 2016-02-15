package services

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

var (
	L = NewLogger(LevelNormal)
)

func init() {
	msg := fmt.Sprintf("Loglevel: %v=%d, %v=%d, %v=%d, %v=%d",
		LevelOff, LevelOff,
		LevelTrace, LevelTrace,
		LevelNormal, LevelNormal,
		LevelWarn, LevelWarn)
	flag.IntVar((*int)(&(L.Level)), "loglevel", int(LevelNormal), msg)
}

type LogLevel int

const (
	LevelOff    = LogLevel(0)
	LevelTrace  = LogLevel(5)
	LevelNormal = LogLevel(10)
	LevelWarn   = LogLevel(15)
)

var (
	LevelNames = map[LogLevel]string{
		LevelOff:    "  Off",
		LevelTrace:  "Trace",
		LevelNormal: "  Log",
		LevelWarn:   " Warn",
	}
)

func (level LogLevel) Name() string {
	if name, ok := LevelNames[level]; ok {
		return name
	} else {
		return fmt.Sprintf(" Lv-%d", level)
	}
}

func (level LogLevel) String() string {
	return strings.TrimSpace(level.Name())
}

type Logger struct {
	Prefix string
	Level  LogLevel
	logger *log.Logger
}

func NewLogger(level LogLevel) *Logger {
	logger := new(Logger)
	logger.Enable(level)
	return logger
}

func (logger *Logger) Enable(level LogLevel) {
	if logger.logger == nil {
		logger.logger = log.New(os.Stdout, "", log.LstdFlags)
	}
	logger.Level = level
}

func (logger *Logger) Enabled() bool {
	return logger != nil && logger.logger != nil && logger.Level > LevelOff
}

func (logger *Logger) LevelEnabled(level LogLevel) bool {
	return logger != nil && logger.logger != nil && logger.Level != LevelOff && level >= logger.Level
}

func (logger *Logger) LogLevelf(level LogLevel, fmt string, v ...interface{}) {
	if logger.LevelEnabled(level) {
		logger.logger.Printf(level.Name()+": "+logger.Prefix+fmt+"\n", v...)
	}
}

func (logger *Logger) Tracef(fmt string, v ...interface{}) {
	logger.LogLevelf(LevelTrace, fmt, v...)
}

func (logger *Logger) Logf(fmt string, v ...interface{}) {
	logger.LogLevelf(LevelNormal, fmt, v...)
}

func (logger *Logger) Warnf(fmt string, v ...interface{}) {
	logger.LogLevelf(LevelWarn, fmt, v...)
}
