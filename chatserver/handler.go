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

type OnlineUser struct {
	ctx common.Context
	*msg.User
	addr     string
	isOnline bool
}

func (o *OnlineUser) IsOnline() bool {
	return o.isOnline
}

func (o *OnlineUser) Addr() string {
	return o.addr
}

var (
	loginHandler     *LoginHandler
	onceLoginHandler = sync.Once{}
)

type LoginHandler struct {
	common.BaseHandler
	onlineUserMap *sync.Map
}

func NewLoginHandler() *LoginHandler {
	onceLoginHandler.Do(func() {
		loginHandler = &LoginHandler{
			onlineUserMap: &sync.Map{},
		}
	})
	return loginHandler
}

func (h *LoginHandler) Do(ctx common.Context, rawMessage json.RawMessage) error {
	message := &msg.LoginMsg{}
	err := json.Unmarshal(rawMessage, message)
	if err != nil {
		log.Println(err)
		h.removeOnlineUser(ctx.RemoteAddr())
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
	h.addOnlineUser(user)
	loginMsg := fmt.Sprintf("login success, now %s, your address is %s", time.Now().String(), user.Addr())
	err = ctx.Write(&common.Message{
		Code:    enum.Echo,
		RawData: loginMsg,
	})
	if err != nil {
		log.Println(err)
		h.removeOnlineUser(user.Addr())
		_ = ctx.Close()
	}
	go h.broadcastMessage(user, user.NickName+"上线了")
	return nil
}

func (h *LoginHandler) OnClose(ctx common.Context) {
	user, ok := GetOnlineUser(ctx.RemoteAddr())
	if !ok {
		return
	}
	h.removeOnlineUser(ctx.RemoteAddr())
	go h.broadcastMessage(user, user.NickName+"下线了")
}

func (h *LoginHandler) addOnlineUser(user *OnlineUser) {
	h.onlineUserMap.Store(user.Addr(), user)
}

func (h *LoginHandler) removeOnlineUser(addr string) {
	user, ok := GetOnlineUser(addr)
	if ok {
		user.isOnline = false
		h.onlineUserMap.Delete(addr)
	}
}

func (h *LoginHandler) broadcastMessage(user *OnlineUser, msg string) {
	users := GetOnlineUsers(1000)
	for i := range users {
		if user.Addr() == users[i].Addr() {
			continue
		}
		err := users[i].ctx.Write(&common.Message{
			Code:    enum.Echo,
			RawData: msg,
		})
		if err != nil {
			h.removeOnlineUser(users[i].Addr())
			_ = users[i].ctx.Close()
		}
	}
}

func GetOnlineUser(addr string) (*OnlineUser, bool) {
	user, ok := loginHandler.onlineUserMap.Load(addr)
	return user.(*OnlineUser), ok
}

func GetOnlineUsers(limit int) []*OnlineUser {
	users := make([]*OnlineUser, 0)
	loginHandler.onlineUserMap.Range(func(key, value interface{}) bool {
		if len(users) > limit {
			return false
		}
		user := value.(*OnlineUser)
		users = append(users, user)
		return true
	})
	return users
}

type onlineUserListHandler struct {
	common.BaseHandler
}

func NewOnlineUserListHandler() *onlineUserListHandler {
	return &onlineUserListHandler{}
}

func (h *onlineUserListHandler) Do(ctx common.Context, _ json.RawMessage) error {
	users := GetOnlineUsers(1000)
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
