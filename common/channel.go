package common

import "net"

type Channel interface {
	Write(*Message) error
	Close() error
	Read() (*RawMessage, error)
}

type SimpleChannelImpl struct {
	coder Codec
	conn  net.Conn
}

func (c *SimpleChannelImpl) Write(msg *Message) error {
	return c.coder.Encode(msg)
}

func (c *SimpleChannelImpl) Close() error {
	return c.conn.Close()
}

func (c *SimpleChannelImpl) Read() (*RawMessage, error) {
	message := &RawMessage{}
	return message, c.coder.Decode(message)
}

func NewSimpleChannel(codec Codec, conn net.Conn) *SimpleChannelImpl {
	return &SimpleChannelImpl{
		coder: codec,
		conn:  conn,
	}
}
