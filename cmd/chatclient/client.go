package main

import (
	"fmt"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/goclient"
	"os"
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
	dispatcher := cli.NewCommandDispatcher(os.Stdin)
	AssertNotError(dispatcher.Register(NewLoginCommand()))
	AssertNotError(dispatcher.Register(NewLogoutCommand()))
	AssertNotError(dispatcher.Register(NewGetUserListCommand()))
	AssertNotError(dispatcher.Register(NewSendCommand()))
	go dispatcher.Dispatch()
	cli.Start()
}
