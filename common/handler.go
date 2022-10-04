package common

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

type Env interface {
	AddHandler(code MessageCode, handler Handler)
	RemoveHandler(code MessageCode)
}

type Handler interface {
	OnMessage(ctx Context, message *RawMessage) error
	OnActive(ctx Context)
	OnClose(ctx Context)
	OnInit(env Env)
	OnRemove(env Env)
}

type BaseHandler struct {
}

func (h *BaseHandler) OnMessage(_ Context, _ *RawMessage) error { return nil }

func (h *BaseHandler) OnActive(_ Context) {}

func (h *BaseHandler) OnClose(_ Context) {}

func (h *BaseHandler) OnInit(_ Env) {}

func (h *BaseHandler) OnRemove(_ Env) {}

type displayHandler struct {
	BaseHandler
	display func(msg string) error
}

func (h *displayHandler) OnMessage(_ Context, message *RawMessage) error {
	msg := ""
	if err := json.Unmarshal(message.RawData, &msg); err != nil {
		return err
	}
	return h.display(msg)
}

func NewDisplayHandler(display func(msg string) error) *displayHandler {
	return &displayHandler{display: display}
}

func (h *displayHandler) ChangeDisplayFunc(display func(msg string) error) {
	h.display = display
}

type pingHandler struct {
	BaseHandler
	code MessageCode
}

func NewPingHandler(pongCode MessageCode) *pingHandler {
	return &pingHandler{code: pongCode}
}

func (h *pingHandler) OnMessage(ctx Context, _ *RawMessage) error {
	return ctx.Write(&Message{
		Code:    h.code,
		RawData: "[pong]",
	})
}

type pongHandler struct {
	connMap *sync.Map
	code    MessageCode
	*time.Ticker
	timeInterval   time.Duration
	maxNoReplyTime time.Duration
	isRemoved      bool
}

type connState struct {
	Context
	lastPongTime time.Time
}

func NewPongHandler(pingCode MessageCode, timeInterval, maxNoReplyTime time.Duration) *pongHandler {
	return &pongHandler{
		connMap:        &sync.Map{},
		code:           pingCode,
		timeInterval:   timeInterval,
		maxNoReplyTime: maxNoReplyTime,
	}
}

func (h *pongHandler) OnMessage(ctx Context, _ *RawMessage) error {
	conn, ok := h.connMap.Load(ctx.RemoteAddr())
	if !ok {
		return nil
	}
	conn.(*connState).lastPongTime = time.Now()
	return nil
}

func (h *pongHandler) OnActive(ctx Context) {
	h.connMap.Store(ctx.RemoteAddr(), &connState{
		Context:      ctx,
		lastPongTime: time.Now(),
	})
}

func (h *pongHandler) OnClose(ctx Context) {
	log.Println("remove inactive connect:" + ctx.RemoteAddr())
	h.connMap.Delete(ctx.RemoteAddr())
}

func (h *pongHandler) OnInit(_ Env) {
	go h.ping()
}

func (h *pongHandler) OnRemove(_ Env) {
	h.isRemoved = true
}

func (h *pongHandler) ping() {
	ticker := time.NewTicker(h.timeInterval)
	defer ticker.Stop()
	h.Ticker = ticker
	for {
		if h.isRemoved {
			break
		}
		<-ticker.C
		h.connMap.Range(func(key, value interface{}) bool {
			state := value.(*connState)
			if time.Now().Sub(state.lastPongTime) > h.maxNoReplyTime {
				h.connMap.Delete(key)
				_ = state.Close()
				log.Println("lose connect " + state.RemoteAddr())
				return true
			}
			err := state.Write(&Message{
				Code:    h.code,
				RawData: "[ping]",
			})
			log.Printf("[ping] remote address: %s\n", state.RemoteAddr())
			if err != nil {
				log.Printf("[ping] error=%s, address=%s\n",
					err.Error(), state.RemoteAddr())
			}
			return true
		})
	}
}
