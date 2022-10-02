package chatserver

import (
	"encoding/json"
	"fmt"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/message/msg"
	"log"
	"strings"
	"sync"
	"time"
)

var (
	onlineUserMap = &sync.Map{}
)

type OnlineUser struct {
	ctx common.Context
	*msg.User
	addr string
	isOnline bool
}

func (o *OnlineUser) IsOnline() bool {
	return o.isOnline
}

func (o *OnlineUser) Addr() string {
	return o.addr
}

func LoginHandler(ctx common.Context, rawMessage json.RawMessage) error {
	message := &msg.LoginMsg{}
	err := json.Unmarshal(rawMessage, message)
	if err != nil {
		log.Println(err)
		removeOnlineUser(ctx.RemoteAddr())
		_ = ctx.Close()
		return err
	}
	user := &OnlineUser{
		ctx: ctx,
		User: &msg.User{
			NickName: message.NickName,
		},
		addr:     ctx.RemoteAddr(),
		isOnline: true,
	}
	addOnlineUser(user)
	loginMsg := fmt.Sprintf("login success, now %s, your address is %s", time.Now().String(), user.Addr())
	err = ctx.Write(&common.Message{
		Code:    enum.Echo,
		RawData: loginMsg,
	})
	if err != nil {
		log.Println(err)
		removeOnlineUser(user.Addr())
		_ = ctx.Close()
	}
	go broadcastUserLoginEvent(user)
	return nil
}

func addOnlineUser(user *OnlineUser) {
	onlineUserMap.Store(user.Addr(), user)
}

func removeOnlineUser(addr string) {
	user, ok := GetOnlineUser(addr)
	if ok {
		user.isOnline = false
		onlineUserMap.Delete(addr)
	}
}

func GetOnlineUser(addr string) (*OnlineUser, bool) {
	user, ok := onlineUserMap.Load(addr)
	return user.(*OnlineUser), ok
}

func getOnlineUsers(num int) []*OnlineUser {
	users := make([]*OnlineUser, 0)
	onlineUserMap.Range(func(key, value interface{}) bool {
		if len(users) > num {
			return false
		}
		user := value.(*OnlineUser)
		users = append(users, user)
		return true
	})
	return users
}

func OnlineUserListHandler(ctx common.Context, _ json.RawMessage) error {
	users := getOnlineUsers(1000)
	builder := &strings.Builder{}
	builder.WriteString(fmt.Sprintf("online user number: %d\n", len(users)))
	for i := range users {
		builder.WriteString(fmt.Sprintf("address=%s, nickname=%s\n", users[i].Addr(), users[i].NickName))
	}
	err := ctx.Write(&common.Message{
		Code:    enum.Echo,
		RawData: builder.String(),
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func broadcastUserLoginEvent(user *OnlineUser) {
	users := getOnlineUsers(1000)
	for i := range users {
		err := users[i].ctx.Write(&common.Message{
			Code:    enum.Echo,
			RawData: user.NickName + "上线了",
		})
		if err != nil {
			removeOnlineUser(users[i].Addr())
			_ = users[i].ctx.Close()
		}
	}
}
