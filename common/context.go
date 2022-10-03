package common

type Context interface {
	RemoteAddr() string
	LocalAddr() string
	AddHandler(code MessageCode, handler Handler)
	RemoveHandler(code MessageCode)
	Channel
}
