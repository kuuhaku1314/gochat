package goclient

import "gochat/common"

type Command struct {
	Command        string
	Alias          []string
	ParseFunc      func(params string) (*common.Message, error)
	UseParseFunc   bool
	LocalParseFunc func(params string) error
	Tips           string
}

type Dispatcher interface {
	Dispatch()
	Register(command *Command) error
}
