package common

type Context interface {
	RemoteAddr() string
	LocalAddr() string
	Env
	Channel
}
