package goserver

import (
	"fmt"
	"gochat/common"
	"net"
	"runtime/debug"
	"sync"
)

type ServerContext struct {
	remoteAddr string
	localAddr  string
	env        common.Env
	common.Channel
	isClosed bool
}

func (s *ServerContext) RemoteAddr() string {
	return s.remoteAddr
}

func (s *ServerContext) LocalAddr() string {
	return s.localAddr
}

func (s *ServerContext) AddHandler(code common.MessageCode, handler common.Handler) {
	s.env.AddHandler(code, handler)
}

func (s *ServerContext) RemoveHandler(code common.MessageCode) {
	s.env.RemoveHandler(code)
}

func (s *ServerContext) Close() error {
	s.isClosed = true
	return s.Channel.Close()
}

type Interceptor interface {
	OnReadAfter(common.Context, *common.RawMessage) error
	OnWriteBefore(common.Context, *common.Message)
	Name() string
}

type Config struct {
	Address string
}

type ChannelWrapper struct {
	common.Channel
	*ServerContext
	*Server
	once sync.Once
	err  error
}

func (c *ChannelWrapper) Write(msg *common.Message) error {
	c.OnWriteBefore(msg)
	return c.Channel.Write(msg)
}

func (c *ChannelWrapper) Close() error {
	c.once.Do(func() {
		c.err = c.Channel.Close()
	})
	return c.err
}

func (c *ChannelWrapper) Read() (*common.RawMessage, error) {
	msg, err := c.Channel.Read()
	if err != nil {
		return msg, err
	}
	if err := c.OnReadAfter(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *ChannelWrapper) OnWriteBefore(msg *common.Message) {
	var curInterceptor Interceptor
	defer func() {
		if err := recover(); err != nil {
			c.Server.logger.Error(fmt.Sprintf("[panic] on write before error, interceptor name=%s, error=%s",
				curInterceptor.Name(), err))
		}
	}()
	for _, interceptor := range c.interceptors {
		curInterceptor = interceptor
		interceptor.OnWriteBefore(c.ServerContext, msg)
	}
}

func (c *ChannelWrapper) OnReadAfter(msg *common.RawMessage) (err error) {
	var curInterceptor Interceptor
	defer func() {
		if e := recover(); e != nil {
			c.Server.logger.Error(fmt.Sprintf("[panic] on read after error, interceptor name=%s, error=%s",
				curInterceptor.Name(), e))
			err = fmt.Errorf("%s", e)
		}
	}()
	for _, interceptor := range c.interceptors {
		curInterceptor = interceptor
		if err = interceptor.OnReadAfter(c.ServerContext, msg); err != nil {
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
		logger:       common.NewConsoleLogger(common.Debug),
	}, nil
}

func (s *Server) AddHandler(code common.MessageCode, handler common.Handler) {
	s.lock.Lock()
	_, ok := s.handlerMap[code]
	if ok {
		s.logger.Fatal("duplicate handler")
	}
	s.handlerMap[code] = handler
	s.lock.Unlock()
	handler.OnInit(s)
}

func (s *Server) RemoveHandler(code common.MessageCode) {
	s.lock.Lock()
	handler, ok := s.handlerMap[code]
	if ok {
		s.logger.Info(fmt.Sprintf("remove handler code=%d", code))
	}
	delete(s.handlerMap, code)
	s.lock.Unlock()
	handler.OnRemove(s)
}

func (s *Server) AddInterceptor(i Interceptor) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.interceptors = append(s.interceptors, i)
}

func (s *Server) Serve() {
	s.logger.Info(fmt.Sprintf("server start serve, bind address=%s", s.address))
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			err = s.listener.Close()
			s.logger.Fatal(err)
		}
		s.logger.Info("new remote client connecting " + conn.RemoteAddr().String())
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	var ctx *ServerContext
	defer func() {
		if err := recover(); err != nil {
			s.logger.Error(fmt.Sprintf("[panic], err=%s, remote address=%s, stack=[%s]",
				err, conn.RemoteAddr(), string(debug.Stack())))
		}
		if ctx != nil {
			_ = ctx.Close()
			s.clientPool.Delete(ctx.RemoteAddr())
			for _, handler := range s.handlerMap {
				handler.OnClose(ctx)
			}
		} else {
			_ = conn.Close()
		}
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
	ctx = &ServerContext{
		remoteAddr: conn.RemoteAddr().String(),
		localAddr:  conn.LocalAddr().String(),
		env:        s,
	}
	ch := &ChannelWrapper{
		Channel:       common.NewSimpleChannel(codec, conn),
		ServerContext: ctx,
		Server:        s,
	}
	ctx.Channel = ch

	s.clientPool.Store(ctx.RemoteAddr(), ctx)
	for _, handler := range s.handlerMap {
		handler.OnActive(ctx)
	}
	for {
		message, err := ctx.Read()
		if err != nil {
			s.logger.Error(err)
			break
		}
		handler, ok := s.handlerMap[message.Code]
		if !ok {
			s.logger.Info(fmt.Sprintf("not found matchable handler, remote address=%s, code=%d",
				ctx.RemoteAddr(), message.Code))
			break
		}
		if err = SafelyDo(handler, ctx, message); err != nil {
			s.logger.Error(err)
		}
		if ctx.isClosed {
			break
		}
	}
}

func SafelyDo(handler common.Handler, ctx common.Context, message *common.RawMessage) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("[panic], err=%s, remote address=%s, stack=%s",
				err, ctx.RemoteAddr(), string(debug.Stack()))
		}
	}()
	return handler.OnMessage(ctx, message)
}
