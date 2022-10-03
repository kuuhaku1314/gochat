package main

import (
	"fmt"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/message/msg"
	"gochat/goclient"
	"time"
)

func main() {
	cli, err := goclient.NewClient("localhost:8080")
	if err != nil {
		fmt.Println(err)
		return
	}
	cli.AddHandler(enum.Ping, common.NewPingHandler(enum.Pong))
	cli.AddHandler(enum.Display, common.NewDisplayHandler(
		func(msg string) error {
			fmt.Println(msg)
			return nil
		}))

	go func() {
		time.Sleep(time.Second * 3)
		fmt.Println("try login")
		cli.SendMessage(&common.Message{
			Code: enum.UserLogin,
			RawData: msg.LoginMsg{
				NickName: "kuuhaku1314",
			},
		})
		time.Sleep(time.Second * 3)
		fmt.Println("try get user list")
		cli.SendMessage(&common.Message{
			Code:    enum.GetOnlineUserList,
			RawData: nil,
		})
	}()
	cli.Start()
}
