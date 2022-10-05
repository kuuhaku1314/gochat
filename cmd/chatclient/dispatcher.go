package main

import (
	"bufio"
	"errors"
	"gochat/common/util"
	"gochat/goclient"
	"io"
	"log"
	"strings"
)

type commandDispatcher struct {
	*bufio.Scanner
	commandMap map[string]*goclient.Command
}

func (c *commandDispatcher) Dispatch() {
	for c.Scan() {
		str := c.Text()
		if len(str) == 0 {
			continue
		}
		arr := strings.SplitN(str, " ", 2)
		command, ok := c.commandMap[arr[0]]
		if !ok {
			log.Println("command not found, you can use [list] command to get command list")
			continue
		}
		params := ""
		if len(arr) > 1 {
			params = arr[1]
		}
		if !command.UseParseFunc {
			if err := command.LocalParseFunc(params); err != nil {
				log.Println(err)
			}
			continue
		}
		message, err := command.ParseFunc(params)
		if err != nil {
			log.Println("parse params error")
			continue
		}
		GetClient().SendMessage(message)
	}
	log.Println("quit dispatcher")
}

func (c *commandDispatcher) Register(command *goclient.Command) error {
	command.Command = strings.TrimSpace(command.Command)
	for i, alias := range command.Alias {
		command.Alias[i] = strings.TrimSpace(alias)
	}
	if len(command.Command) == 0 {
		return errors.New("invalid command")
	}
	if command.UseParseFunc {
		if command.ParseFunc == nil {
			return errors.New("ParseFunc not found")
		}
		if command.LocalParseFunc != nil {
			return errors.New("LocalParseFunc should be nil")
		}
	} else {
		if command.ParseFunc != nil {
			return errors.New("ParseFunc should be nil")
		}
		if command.LocalParseFunc == nil {
			return errors.New("LocalParseFunc not found")
		}
	}
	_, ok := c.commandMap[command.Command]
	if ok {
		return errors.New("duplicate command")
	}
	c.commandMap[command.Command] = command
	for _, alias := range command.Alias {
		_, ok = c.commandMap[alias]
		if ok {
			return errors.New("duplicate alias")
		}
		c.commandMap[alias] = command
	}
	return nil
}

func NewCommandDispatcher(reader io.Reader) goclient.Dispatcher {
	dispatcher := &commandDispatcher{
		Scanner:    bufio.NewScanner(reader),
		commandMap: make(map[string]*goclient.Command),
	}
	listCommand := &goclient.Command{
		Command: "list",
		LocalParseFunc: func(params string) error {
			displayTips := false
			if strings.TrimSpace(params) == "-all" {
				displayTips = true
			}
			sb := &strings.Builder{}
			sb.WriteString("now command list:\n")
			for _, command := range dispatcher.commandMap {
				sb.WriteString("command:[")
				sb.WriteString(command.Command)
				sb.WriteString("]\n")
				if displayTips {
					sb.WriteString(command.Tips)
					sb.WriteString("\n")
				}
			}
			log.Println(sb.String())
			return nil
		},
		Tips: "display all command info, use option -all can display command tips",
	}
	helpCommand := &goclient.Command{
		Command: "help",
		LocalParseFunc: func(params string) error {
			str := strings.TrimSpace(params)
			if str == "" {
				log.Println("please add command after help")
				return nil
			}
			command, ok := dispatcher.commandMap[str]
			if !ok {
				log.Println("not found command")
				return nil
			}
			log.Println(command.Tips)
			return nil
		},
		Tips: "show command tips, use likes help [command], example: help help",
	}
	exitCommand := &goclient.Command{
		Command: "exit",
		LocalParseFunc: func(params string) error {
			log.Println("exit client success")
			_ = GetClient().Close()
			return nil
		},
		Tips: "exit process",
	}
	util.AssertNotError(dispatcher.Register(listCommand))
	util.AssertNotError(dispatcher.Register(helpCommand))
	util.AssertNotError(dispatcher.Register(exitCommand))
	return dispatcher
}
