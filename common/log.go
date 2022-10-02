package common

import (
	"fmt"
	"os"
)

type Logger interface {
	Info(...interface{})
	Error(...interface{})
	Fatal(...interface{})
}

type ConsoleLogger struct{}

func (log *ConsoleLogger) Info(msg ...interface{}) {
	fmt.Println(msg)
}

func (log *ConsoleLogger) Error(msg ...interface{}) {
	println(msg)
}

func (log *ConsoleLogger) Fatal(msg ...interface{}) {
	println(msg)
	os.Exit(-1)
}
