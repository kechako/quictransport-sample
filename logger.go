package main

import (
	"log"
	"os"
)

type LogLevel int

const (
	Debug LogLevel = iota
	Info
	Warn
	Error
)

type Logger interface {
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Warn(v ...interface{})
	Warnf(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
}

type nopLogger struct{}

func (l nopLogger) Debug(v ...interface{})                 {}
func (l nopLogger) Debugf(format string, v ...interface{}) {}
func (l nopLogger) Info(v ...interface{})                  {}
func (l nopLogger) Infof(format string, v ...interface{})  {}
func (l nopLogger) Warn(v ...interface{})                  {}
func (l nopLogger) Warnf(format string, v ...interface{})  {}
func (l nopLogger) Error(v ...interface{})                 {}
func (l nopLogger) Errorf(format string, v ...interface{}) {}

type stdLogger struct {
	level LogLevel

	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	error *log.Logger
}

func NewStdLogger(level LogLevel) Logger {
	return &stdLogger{
		level: level,
		debug: log.New(os.Stdout, "[DEBUG] ", log.LstdFlags|log.Lmsgprefix),
		info:  log.New(os.Stdout, "[INFO ] ", log.LstdFlags|log.Lmsgprefix),
		warn:  log.New(os.Stderr, "[WARN ] ", log.LstdFlags|log.Lmsgprefix),
		error: log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lmsgprefix),
	}
}

func (l *stdLogger) Debug(v ...interface{}) {
	if l.level > Debug {
		return
	}
	l.debug.Print(v...)
}

func (l *stdLogger) Debugf(format string, v ...interface{}) {
	if l.level > Debug {
		return
	}
	l.debug.Printf(format, v...)
}

func (l *stdLogger) Info(v ...interface{}) {
	if l.level > Info {
		return
	}
	l.info.Print(v...)
}

func (l *stdLogger) Infof(format string, v ...interface{}) {
	if l.level > Info {
		return
	}
	l.info.Printf(format, v...)
}

func (l *stdLogger) Warn(v ...interface{}) {
	if l.level > Warn {
		return
	}
	l.warn.Print(v...)
}

func (l *stdLogger) Warnf(format string, v ...interface{}) {
	if l.level > Warn {
		return
	}
	l.warn.Printf(format, v...)
}

func (l *stdLogger) Error(v ...interface{}) {
	l.error.Print(v...)
}

func (l *stdLogger) Errorf(format string, v ...interface{}) {
	l.error.Printf(format, v...)
}
