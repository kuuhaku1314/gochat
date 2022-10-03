package chatserver

import (
	"errors"
	"fmt"
	"gochat/common"
	"net"
	"sync"
)

type ServerContext struct {
	remoteAddr string
	localAddr  string
	common.Channel
	isClosed bool
}

func (s *ServerContext) RemoteAddr() string {
	return s.remoteAddr
}

func (s *ServerContext) LocalAddr() string {
	return s.localAddr
}

func (s *ServerContext) Close() error {
	s.isClosed = true
	return s.Channel.Close()
}

type Interceptor interface {
	DoBefore(common.Context, *common.RawMessage) error
	DoAfter(common.Context, *common.Message)
	Name() string
}

type Config struct {
	Address string
}

type ChannelWrapper struct {
	common.Channel
	*ServerContext
	*Server
}

func (c *ChannelWrapper) Write(msg *common.Message) error {
	c.doAfter(msg)
	return c.Channel.Write(msg)
}

func (c *ChannelWrapper) Close() error {
	return c.Channel.Close()
}

func (c *ChannelWrapper) Read() (*common.RawMessage, error) {
	msg, err := c.Channel.Read()
	if err != nil {
		return msg, err
	}
	if err := c.doBefore(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *ChannelWrapper) doAfter(msg *common.Message) {
	var curInterceptor Interceptor
	defer func() {
		if err := recover(); err != nil {
			c.Server.logger.Error(fmt.Sprintf("[panic] do after error, interceptor name=%s, error=%s",
				curInterceptor.Name(), err))
		}
	}()
	for _, interceptor := range c.interceptors {
		curInterceptor = interceptor
		interceptor.DoAfter(c.ServerContext, msg)
	}
}

func (c *ChannelWrapper) doBefore(msg *common.RawMessage) (err error) {
	var curInterceptor Interceptor
	defer func() {
		if e := recover(); e != nil {
			c.Server.logger.Error(fmt.Sprintf("[panic] do before error, interceptor name=%s, error=%s",
				curInterceptor.Name(), e))
			err = errors.New(fmt.Sprintf("%s", e))
		}
	}()
	for _, interceptor := range c.interceptors {
		curInterceptor = interceptor
		if err = interceptor.DoBefore(c.ServerContext, msg); err != nil {
			return err
		}
	}
	return nil
}

type Server struct {
	address      string
	listener     net.Listener
	clientPool   *sync.Map
	lock         sync.Mutex
	handlerMap   map[common.MessageCode]common.Handler
	interceptors []Interceptor
	logger       common.Logger
}

func NewServer(address string) (*Server, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	return &Server{
		address:      address,
		listener:     listener,
		clientPool:   &sync.Map{},
		lock:         sync.Mutex{},
		handlerMap:   make(map[common.MessageCode]common.Handler),
		interceptors: nil,
		logger:       &common.ConsoleLogger{},
	}, nil
}

func (s *Server) AddHandler(code common.MessageCode, handler common.Handler) {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, ok := s.handlerMap[code]
	if ok {
		s.logger.Fatal("duplicate handler")
	}
	s.handlerMap[code] = handler
	handler.OnInit()
}

func (s *Server) AddInterceptor(i Interceptor) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.interceptors = append(s.interceptors, i)
}

func (s *Server) Serve() {
	s.logger.Info(fmt.Sprintf("chatserver start, bind address=%s", s.address))
	for true {
		conn, err := s.listener.Accept()
		if err != nil {
			err = s.listener.Close()
			s.logger.Fatal(err)
		}
		s.logger.Info("remote client connecting " + conn.RemoteAddr().String())
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Error(fmt.Sprintf("[panic], err=%s, remote address=%s", err, conn.RemoteAddr()))
		}
		_ = conn.Close()
	}()
	header, err := common.ReadHeader(conn)
	if err != nil {
		s.logger.Error("read header error", err)
		return
	}
	if err = header.Validate(); err != nil {
		s.logger.Error("validate header error", err)
		return
	}
	codec, err := common.GetCodec(int8(header.CodecType), conn)
	if err != nil {
		s.logger.Error("get codec error", err)
		return
	}
	s.logger.Info(fmt.Sprintf("connecting completed, remote address=%s", conn.RemoteAddr()))
	s.clientPool.Store(conn.RemoteAddr(), conn)
	ctx := &ServerContext{
		remoteAddr: conn.RemoteAddr().String(),
		localAddr:  conn.LocalAddr().String(),
	}
	ch := &ChannelWrapper{
		Channel:       common.NewSimpleChannel(codec, conn),
		ServerContext: ctx,
		Server:        s,
	}
	ctx.Channel = ch
	for _, handler := range s.handlerMap {
		handler.OnActive(ctx)
	}
	for {
		message, err := ctx.Read()
		if err != nil {
			s.logger.Info(err)
			break
		}
		handler, ok := s.handlerMap[message.Code]
		if !ok {
			s.logger.Info(fmt.Sprintf("not have matchable handler, remote address=%s",
				conn.RemoteAddr().String()))
			break
		}
		if err = handler.Do(ctx, message.RawData); err != nil {
			s.logger.Error(err)
		}
		if ctx.isClosed {
			break
		}
	}
	s.clientPool.Delete(conn.RemoteAddr())
	for _, handler := range s.handlerMap {
		handler.OnClose(ctx)
	}
}
