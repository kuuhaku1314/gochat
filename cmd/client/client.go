package main

import (
	"fmt"
	"gochat/chatclient"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/message/msg"
	"time"
)

func main() {
	newClient, err := chatclient.NewClient("localhost:8080")
	if err != nil {
		fmt.Println(err)
		return
	}
	newClient.AddHandler(enum.Echo, common.NewEchoHandler(func(msg string) error {
		fmt.Println(msg)
		return nil
	}))
	go func() {
		time.Sleep(time.Second*2)
		fmt.Println("try login")
		err := newClient.Write(&common.Message{
			Code:    enum.UserLogin,
			RawData: msg.User{
				NickName: "TOO",
			},
		})
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("try get user list")
		err = newClient.Write(&common.Message{
			Code:    enum.GetOnlineUserList,
			RawData: nil,
		})
		if err != nil {
			fmt.Println(err)
			return
		}
	}()
	newClient.Start()
}
