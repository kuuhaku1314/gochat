package chatclient

import (
	"encoding/json"
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
	conn       net.Conn
	handlerMap map[common.MessageCode]func(ctx common.Context, message json.RawMessage) error
	codec      common.Codec
	logger     common.Logger
	isClosed   bool
	once sync.Once
}

func NewClient(address string) (*Client, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	client := &Client{
		conn:       conn,
		handlerMap: make(map[common.MessageCode]func(ctx common.Context, message json.RawMessage) error),
		codec:      common.NewJsonCodec(conn),
		logger:     &common.ConsoleLogger{},
	}
	header := common.NewHeader(common.JsonCodecType)
	_, err = client.conn.Write(header.Bytes())
	if err != nil {
		return nil, err
	}
	client.logger.Info(fmt.Sprintf("start chatclient success, local address=%s", conn.LocalAddr().String()))
	return client, nil
}

func (c *Client) AddHandler(code common.MessageCode, handler func(ctx common.Context, message json.RawMessage) error) {
	c.handlerMap[code] = handler
}

func (c *Client) Write(msg *common.Message) error {
	return c.codec.Encode(msg)
}

func (c *Client) Close() error {
	c.isClosed = true
	var err error
	c.once.Do(func() {
		err = c.conn.Close()
	})
	return err
}

func (c *Client) SetCloseFlag() {
	c.isClosed = true
}

func (c *Client) Start() {
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
		if err := handler(ctx, message.RawData); err != nil {
			log.Println(err)
			continue
		}
	}
	_ = c.Close()
}
