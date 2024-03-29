package main

import (
	"fmt"
	"gochat/cmd/chatserver/handler"
	"gochat/cmd/chatserver/interceptor"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/util"
	"gochat/goserver"
	"log"
	"time"
)

func main() {
	log.Println("输入要监听的IP端口，不输入默认为localhost:8080")
	address := util.ScanAddress("localhost:8080")
	s, err := goserver.NewServer(address)
	if err != nil {
		fmt.Println(err)
		time.Sleep(time.Second * 5)
		return
	}
	s.AddInterceptor(interceptor.NewCountInterceptor())
	s.AddHandler(enum.Display, common.NewDisplayHandler(
		func(msg string) error {
			log.Println(msg)
			return nil
		}))
	s.AddHandler(enum.Pong, common.NewPongHandler(enum.Ping, time.Second*15, time.Minute))
	s.AddHandler(handler.UserHandlerCode, handler.NewUserHandler())
	s.Serve()
}
