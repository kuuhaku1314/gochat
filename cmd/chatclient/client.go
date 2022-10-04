package main

import (
	"fmt"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/util"
	"gochat/goclient"
	"os"
	"time"
)

func main() {
	fmt.Println("输入要连接的IP端口，不输入默认为localhost:8080")
	address := util.ScanAddress("localhost:8080")
	cli, err := goclient.NewClient(address)
	if err != nil {
		fmt.Println(err)
		time.Sleep(time.Second * 5)
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
