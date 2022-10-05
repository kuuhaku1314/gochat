package goclient

import (
	"bufio"
	"errors"
	"fmt"
	"gochat/common"
	"gochat/common/util"
	"io"
	"log"
	"net"
	"strings"
	"sync"
)

type ClientContext struct {
	remoteAddr string
	localAddr  string
	client     *Client
	common.Channel
}

func (s *ClientContext) RemoteAddr() string {
	return s.remoteAddr
}

func (s *ClientContext) LocalAddr() string {
	return s.localAddr
}

func (s *ClientContext) AddHandler(code common.MessageCode, handler common.Handler) {
	s.client.AddHandler(code, handler)
}

func (s *ClientContext) RemoveHandler(code common.MessageCode) {
	s.client.RemoveHandler(code)
}

type Client struct {
	conn         net.Conn
	handlerMap   map[common.MessageCode]common.Handler
	codec        common.Codec
	logger       common.Logger
	isClosed     bool
	once         sync.Once
	messageQueue chan *common.Message
}

func NewClient(address string) (*Client, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	client := &Client{
		conn:         conn,
		handlerMap:   make(map[common.MessageCode]common.Handler),
		codec:        common.NewJsonCodec(conn),
		logger:       common.NewConsoleLogger(common.Debug),
		messageQueue: make(chan *common.Message, 1000),
	}
	header := common.NewHeader(common.JsonCodecType)
	_, err = client.conn.Write(header.Bytes())
	if err != nil {
		return nil, err
	}
	client.logger.Info(fmt.Sprintf("start client success, local address=%s", conn.LocalAddr().String()))
	return client, nil
}

func (c *Client) AddHandler(code common.MessageCode, handler common.Handler) {
	c.handlerMap[code] = handler
	handler.OnInit(c)
}

func (c *Client) Close() error {
	c.isClosed = true
	var err error
	c.once.Do(func() {
		err = c.conn.Close()
		close(c.messageQueue)
	})
	return err
}

func (c *Client) SetCloseFlag() {
	c.isClosed = true
}

func (c *Client) RemoveHandler(code common.MessageCode) {
	handler, ok := c.handlerMap[code]
	if ok {
		c.logger.Info(fmt.Sprintf("remove handler code=%d", code))
	}
	delete(c.handlerMap, code)
	handler.OnRemove(c)
}

func (c *Client) Start() {
	ctx := &ClientContext{
		remoteAddr: c.conn.RemoteAddr().String(),
		localAddr:  c.conn.LocalAddr().String(),
		Channel:    common.NewSimpleChannel(c.codec, c.conn),
	}
	for _, handler := range c.handlerMap {
		handler.OnActive(ctx)
	}
	go func() {
		c.logger.Info("start pull message")
		for msg := range c.messageQueue {
			if c.isClosed {
				return
			}
			if msg == nil {
				continue
			}
			if err := ctx.Write(msg); err != nil {
				log.Println(err)
			}
		}
		c.logger.Info("end pull message")
	}()
	for {
		if c.isClosed {
			break
		}
		message, err := ctx.Read()
		if err != nil {
			// 非主动关闭
			if !c.isClosed {
				c.logger.Error(err)
			}
			break
		}
		handler, ok := c.handlerMap[message.Code]
		if !ok {
			log.Println("unknown message", message)
			continue
		}
		if err := handler.OnMessage(ctx, message); err != nil {
			log.Println(err)
			continue
		}
	}
	_ = c.Close()
	for _, handler := range c.handlerMap {
		handler.OnClose(ctx)
	}
}

func (c *Client) SendMessage(message *common.Message) {
	c.messageQueue <- message
}

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

type commandDispatcher struct {
	*bufio.Scanner
	client     *Client
	commandMap map[string]*Command
}

func (c *commandDispatcher) Dispatch() {
	for c.Scan() {
		str := c.Text()
		if len(str) == 0 {
			c.client.logger.Error("invalid input")
			continue
		}
		arr := strings.SplitN(str, " ", 2)
		command, ok := c.commandMap[arr[0]]
		if !ok {
			c.client.logger.Error("command not found, you can use [list] command to get command list")
			continue
		}
		params := ""
		if len(arr) > 1 {
			params = arr[1]
		}
		if !command.UseParseFunc {
			if err := command.LocalParseFunc(params); err != nil {
				c.client.logger.Error(err)
			}
			continue
		}
		message, err := command.ParseFunc(params)
		if err != nil {
			c.client.logger.Error("parse params error")
			continue
		}
		c.client.SendMessage(message)
	}
	c.client.logger.Info("quit dispatcher")
}

func (c *commandDispatcher) Register(command *Command) error {
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

func (c *Client) NewCommandDispatcher(reader io.Reader) Dispatcher {
	dispatcher := &commandDispatcher{
		Scanner:    bufio.NewScanner(reader),
		client:     c,
		commandMap: make(map[string]*Command),
	}
	listCommand := &Command{
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
	helpCommand := &Command{
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
	exitCommand := &Command{
		Command: "exit",
		LocalParseFunc: func(params string) error {
			log.Println("exit client success")
			_ = c.Close()
			return nil
		},
		Tips: "exit process",
	}
	util.AssertNotError(dispatcher.Register(listCommand))
	util.AssertNotError(dispatcher.Register(helpCommand))
	util.AssertNotError(dispatcher.Register(exitCommand))
	return dispatcher
}
