package main

import (
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/util"
	"gochat/goclient"
	"log"
	"os"
	"time"
)

func main() {
	log.Println("输入要连接的服务器IP端口，不输入默认为localhost:8080")
	address := util.ScanAddress("localhost:8080")
	cli, err := goclient.NewClient(address)
	if err != nil {
		log.Println(err)
		time.Sleep(time.Second * 5)
		return
	}
	cli.SetDispatcher(NewCommandDispatcher(cli, os.Stdin))

	cli.AddHandler(enum.Ping, common.NewPingHandler(enum.Pong))
	cli.AddHandler(enum.Display, common.NewDisplayHandler(
		func(msg string) error {
			log.Println(msg)
			return nil
		}))
	cli.AddHandler(enum.FileTransfer, NewFileTransferHandler(cli, time.Second*90))
	util.AssertNotError(cli.Register(NewLoginCommand()))
	util.AssertNotError(cli.Register(NewLogoutCommand()))
	util.AssertNotError(cli.Register(NewGetUserListCommand()))
	util.AssertNotError(cli.Register(NewSendCommand()))
	cli.Start()
}
