package interceptor

import (
	"gochat/common"
	"log"
	"sync/atomic"
)

type countInterceptor struct {
	sendMsgNum    int64
	receiveMsgNum int64
}

func NewCountInterceptor() *countInterceptor {
	return &countInterceptor{}
}

func (i *countInterceptor) DoBefore(common.Context, *common.RawMessage) error {
	log.Printf("receive message count=%d", atomic.AddInt64(&i.receiveMsgNum, 1))
	return nil
}

func (i *countInterceptor) DoAfter(common.Context, *common.Message) {
	log.Printf("send message count=%d", atomic.AddInt64(&i.sendMsgNum, 1))
}

func (i *countInterceptor) Name() string {
	return "CountInterceptor"
}
