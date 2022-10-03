package main

import (
	"fmt"
	"gochat/chatserver"
	"gochat/common"
	"gochat/common/message/enum"
)

func main() {
	s, err := chatserver.NewServer("localhost:8080")
	if err != nil {
		fmt.Println(err)
		return
	}
	s.AddInterceptor(chatserver.NewCountInterceptor())
	s.AddHandler(enum.Echo, common.NewEchoHandler(
		func(msg string) error {
			fmt.Println(msg)
			return nil
		}))
	s.AddHandler(enum.Pong, common.NewPongHandler(enum.Ping))
	s.AddHandler(enum.UserLogin, chatserver.NewLoginHandler())
	s.AddHandler(enum.GetOnlineUserList, chatserver.NewOnlineUserListHandler())
	s.Serve()
}
