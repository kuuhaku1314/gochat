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
	s.AddHandler(enum.Echo, common.NewEchoHandler(func(msg string) error {
		fmt.Println(msg)
		return nil
	}))
	s.AddHandler(enum.UserLogin, chatserver.LoginHandler)
	s.AddHandler(enum.GetOnlineUserList, chatserver.OnlineUserListHandler)
	s.Serve()
}