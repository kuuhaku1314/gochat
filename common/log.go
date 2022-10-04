package common

import (
	"fmt"
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

func (log *ConsoleLogger) Debug(msg ...interface{}) {
	if log.level > Debug {
		return
	}
	fmt.Println(msg)
}

func (log *ConsoleLogger) Info(msg ...interface{}) {
	if log.level > Info {
		return
	}
	fmt.Println(msg)
}

func (log *ConsoleLogger) Error(msg ...interface{}) {
	if log.level > Error {
		return
	}
	fmt.Println(msg)
}

func (log *ConsoleLogger) Fatal(msg ...interface{}) {
	fmt.Println(msg)
	os.Exit(-1)
}
