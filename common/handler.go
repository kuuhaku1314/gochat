package common

import (
	"encoding/json"
)

type Handler interface {
	Do(ctx Context, message json.RawMessage) error
}

type EchoHandler struct {
	display func(msg string) error
}

func (h *EchoHandler) Do(ctx Context, message json.RawMessage) error {
	msg := ""
	if err := json.Unmarshal(message, &msg); err != nil {
		return err
	}
	return h.display(msg)
}

func NewEchoHandler(display func(msg string) error) *EchoHandler {
	return &EchoHandler{display: display}
}

func HeartbeatHandler() {

}
