package main

import (
	"fmt"
	"gochat/cmd/chatserver/handler"
	"gochat/cmd/chatserver/interceptor"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/goserver"
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
	s.AddHandler(enum.Pong, common.NewPongHandler(enum.Ping))
	s.AddHandler(enum.UserLogin, handler.NewLoginHandler())
	s.AddHandler(enum.GetOnlineUserList, handler.NewOnlineUserListHandler())
	s.Serve()
}
