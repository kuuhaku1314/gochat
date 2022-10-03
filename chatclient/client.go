package chatclient

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
	common.Channel
}

func (s *ClientContext) RemoteAddr() string {
	return s.remoteAddr
}

func (s *ClientContext) LocalAddr() string {
	return s.localAddr
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
		logger:       &common.ConsoleLogger{},
		messageQueue: make(chan *common.Message),
	}
	header := common.NewHeader(common.JsonCodecType)
	_, err = client.conn.Write(header.Bytes())
	if err != nil {
		return nil, err
	}
	client.logger.Info(fmt.Sprintf("start chatclient success, local address=%s", conn.LocalAddr().String()))
	return client, nil
}

func (c *Client) AddHandler(code common.MessageCode, handler common.Handler) {
	c.handlerMap[code] = handler
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

func (c *Client) Start() {
	go func() {
		for {
			msg := <-c.messageQueue
			if msg == nil {
				continue
			}
			err := c.write(msg)
			if err != nil {
				log.Println(err)
			}
		}
	}()
	for {
		if c.isClosed {
			break
		}
		message := &common.RawMessage{}
		if err := c.codec.Decode(message); err != nil {
			log.Println(err)
			break
		}
		handler, ok := c.handlerMap[message.Code]
		if !ok {
			log.Println("unknown message", message)
			continue
		}
		ctx := &ClientContext{
			remoteAddr: c.conn.RemoteAddr().String(),
			localAddr:  c.conn.LocalAddr().String(),
			Channel:    common.NewSimpleChannel(c.codec, c.conn),
		}
		if err := handler.Do(ctx, message.RawData); err != nil {
			log.Println(err)
			continue
		}
	}
	_ = c.Close()
}

func (c *Client) SendMessage(message *common.Message) {
	c.messageQueue <- message
}
