package main

import (
	"fmt"
	"gochat/cmd/chatserver/handler"
	"gochat/cmd/chatserver/interceptor"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/goserver"
	"time"
)

func main() {
	s, err := goserver.NewServer("localhost:8080")
	if err != nil {
		fmt.Println(err)
		return
	}
	s.AddInterceptor(interceptor.NewCountInterceptor())
	s.AddHandler(enum.Display, common.NewDisplayHandler(
		func(msg string) error {
			fmt.Println(msg)
			return nil
		}))
	s.AddHandler(enum.Pong, common.NewPongHandler(enum.Ping, time.Second*15, time.Minute))
	s.AddHandler(handler.UserHandlerCode, handler.GetUserHandler())
	s.Serve()
}
