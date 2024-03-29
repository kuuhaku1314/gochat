package goclient

import (
	"fmt"
	"gochat/common"
	"log"
	"net"
	"sync"
	"time"
)

type ClientContext struct {
	remoteAddr string
	localAddr  string
	client     *Client
	common.Channel
}

func (ctx *ClientContext) RemoteAddr() string {
	return ctx.remoteAddr
}

func (ctx *ClientContext) LocalAddr() string {
	return ctx.localAddr
}

func (ctx *ClientContext) AddHandler(code common.MessageCode, handler common.Handler) {
	ctx.client.AddHandler(code, handler)
}

func (ctx *ClientContext) RemoveHandler(code common.MessageCode) {
	ctx.client.RemoveHandler(code)
}

type Client struct {
	conn         net.Conn
	handlerMap   map[common.MessageCode]common.Handler
	codec        common.Codec
	logger       common.Logger
	isClosed     bool
	once         *sync.Once
	messageQueue chan *common.Message
	dispatcher   Dispatcher
	lock         *sync.Mutex
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
		once:         &sync.Once{},
		messageQueue: make(chan *common.Message, 1000),
		lock:         &sync.Mutex{},
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
	c.lock.Lock()
	_, ok := c.handlerMap[code]
	if ok {
		c.logger.Fatal("duplicate handler")
	}
	c.handlerMap[code] = handler
	c.lock.Unlock()
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

func (c *Client) SetDispatcher(dispatcher Dispatcher) {
	c.dispatcher = dispatcher
}

func (c *Client) Register(command *Command) error {
	if c.dispatcher == nil {
		c.logger.Fatal("dispatcher is not set")
	}
	return c.dispatcher.Register(command)
}

func (c *Client) RemoveHandler(code common.MessageCode) {
	c.lock.Lock()
	handler, ok := c.handlerMap[code]
	if !ok {
		c.logger.Error(fmt.Sprintf("not found handler code=%d", code))
		c.lock.Unlock()
		return
	}
	c.logger.Info(fmt.Sprintf("remove handler code=%d", code))
	delete(c.handlerMap, code)
	c.lock.Unlock()
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
	go c.dispatcher.Dispatch()
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
		c.logger.Info("client is closed, end pull message")
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
	log.Println("client is closing")
	_ = c.Close()
	for _, handler := range c.handlerMap {
		handler.OnClose(ctx)
	}
	log.Println("closing success")
	time.Sleep(time.Second * 3)
}

func (c *Client) SendMessage(message *common.Message) {
	c.messageQueue <- message
}
