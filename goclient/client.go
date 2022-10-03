package goclient

import (
	"fmt"
	"gochat/common"
	"log"
	"net"
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
		messageQueue: make(chan *common.Message),
	}
	header := common.NewHeader(common.JsonCodecType)
	_, err = client.conn.Write(header.Bytes())
	if err != nil {
		return nil, err
	}
	client.logger.Info(fmt.Sprintf("start goclient success, local address=%s", conn.LocalAddr().String()))
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

func (c *Client) write(msg *common.Message) error {
	return c.codec.Encode(msg)
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
		for {
			if c.isClosed {
				return
			}
			msg := <-c.messageQueue
			if msg == nil {
				continue
			}
			err := ctx.Write(msg)
			if err != nil {
				log.Println(err)
			}
		}
	}()
	for {
		if c.isClosed {
			break
		}
		message, err := ctx.Read()
		if err != nil {
			c.logger.Error(err)
			break
		}
		handler, ok := c.handlerMap[message.Code]
		if !ok {
			log.Println("unknown message", message)
			continue
		}
		if err := handler.Do(ctx, message); err != nil {
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
