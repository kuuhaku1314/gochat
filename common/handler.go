package common

import (
	"encoding/json"
)

func NewEchoHandler(display func(msg string) error) func(ctx Context, message json.RawMessage) error {
	return func(ctx Context, message json.RawMessage) error {
		msg := ""
		err := json.Unmarshal(message, &msg)
		if err != nil {
			return err
		}
		return display(msg)
	}
}
