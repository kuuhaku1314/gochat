package common

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

type Handler interface {
	Do(ctx Context, message json.RawMessage) error
	OnActive(ctx Context)
	OnClose(ctx Context)
	OnInit()
}

type BaseHandler struct {
}

func (h *BaseHandler) Do(ctx Context, message json.RawMessage) {}

func (h *BaseHandler) OnActive(ctx Context) {}

func (h *BaseHandler) OnClose(ctx Context) {}

func (h *BaseHandler) OnInit() {}

type echoHandler struct {
	BaseHandler
	display func(msg string) error
}

func (h *echoHandler) Do(ctx Context, message json.RawMessage) error {
	msg := ""
	if err := json.Unmarshal(message, &msg); err != nil {
		return err
	}
	return h.display(msg)
}

func NewEchoHandler(display func(msg string) error) *echoHandler {
	return &echoHandler{display: display}
}

func (h *echoHandler) ChangeDisplayFunc(display func(msg string) error) {
	h.display = display
}

type pingHandler struct {
	BaseHandler
	code MessageCode
}

func NewPingHandler(pongCode MessageCode) *pingHandler {
	return &pingHandler{code: pongCode}
}

func (h *pingHandler) Do(ctx Context, _ json.RawMessage) error {
	log.Println("[pong]")
	return ctx.Write(&Message{
		Code:    h.code,
		RawData: "[pong]",
	})
}

type pongHandler struct {
	connMap *sync.Map
	code    MessageCode
	*time.Ticker
}

type connState struct {
	Context
	lastPongTime time.Time
}

func NewPongHandler(pingCode MessageCode) *pongHandler {
	return &pongHandler{
		connMap: &sync.Map{},
		code:    pingCode,
	}
}

func (h *pongHandler) Do(ctx Context, _ json.RawMessage) error {
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
	h.connMap.Delete(ctx.RemoteAddr())
}

func (h *pongHandler) OnInit() {
	go h.ping()
}

func (h *pongHandler) ping() {
	ticker := time.NewTicker(time.Second * 15)
	h.Ticker = ticker
	for {
		<-ticker.C
		h.connMap.Range(func(key, value interface{}) bool {
			state := value.(*connState)
			if time.Now().Sub(state.lastPongTime).Seconds() > 60 {
				h.connMap.Delete(key)
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
