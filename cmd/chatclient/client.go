package main

import (
	"fmt"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/util"
	"gochat/goclient"
	"log"
	"os"
	"time"
)

var client *goclient.Client

func GetClient() *goclient.Client {
	return client
}

func main() {
	fmt.Println("输入要连接的服务器IP端口，不输入默认为localhost:8080")
	address := util.ScanAddress("localhost:8080")
	cli, err := goclient.NewClient(address)
	if err != nil {
		fmt.Println(err)
		time.Sleep(time.Second * 5)
		return
	}
	client = cli
	cli.AddHandler(enum.Ping, common.NewPingHandler(enum.Pong))
	cli.AddHandler(enum.Display, common.NewDisplayHandler(
		func(msg string) error {
			log.Println(msg)
			return nil
		}))
	transferHandler := NewFileTransferHandler(time.Second * 60)
	cli.AddHandler(enum.FileTransfer, transferHandler)
	dispatcher := cli.NewCommandDispatcher(os.Stdin)
	util.AssertNotError(dispatcher.Register(NewLoginCommand()))
	util.AssertNotError(dispatcher.Register(NewLogoutCommand()))
	util.AssertNotError(dispatcher.Register(NewGetUserListCommand()))
	util.AssertNotError(dispatcher.Register(NewSendCommand()))
	for _, command := range transferHandler.Commands() {
		util.AssertNotError(dispatcher.Register(command))
	}
	go dispatcher.Dispatch()
	cli.Start()
}
