package common

import (
	"log"
	"os"
)

type LogLevel int8

const (
	Debug LogLevel = iota + 1
	Info
	Error
)

type Logger interface {
	Debug(...interface{})
	Info(...interface{})
	Error(...interface{})
	Fatal(...interface{})
}

type ConsoleLogger struct {
	level LogLevel
}

func NewConsoleLogger(level LogLevel) *ConsoleLogger {
	return &ConsoleLogger{level: level}
}

func (l *ConsoleLogger) Debug(msg ...interface{}) {
	if l.level > Debug {
		return
	}
	log.Println(msg)
}

func (l *ConsoleLogger) Info(msg ...interface{}) {
	if l.level > Info {
		return
	}
	log.Println(msg)
}

func (l *ConsoleLogger) Error(msg ...interface{}) {
	if l.level > Error {
		return
	}
	log.Println(msg)
}

func (l *ConsoleLogger) Fatal(msg ...interface{}) {
	log.Println(msg)
	os.Exit(-1)
}
