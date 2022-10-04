package handler

import (
	"encoding/json"
	"fmt"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/message/msg"
	"gochat/goserver"
	"log"
	"strings"
	"sync"
	"time"
)

type OnlineUser struct {
	ctx  common.Context
	user *msg.User
	addr string
}

func (o *OnlineUser) Addr() string {
	return o.addr
}

func (o *OnlineUser) NikeName() string {
	return o.user.NickName
}

const UserHandlerCode common.MessageCode = -100

var (
	uh              *userHandler
	userHandlerOnce = &sync.Once{}
)

// UserHandler 把用户行为聚合到一个Handler里管理
type userHandler struct {
	onlineUserMap *sync.Map
	handlerMap    map[common.MessageCode]common.Handler
}

func GetUserHandler() *userHandler {
	userHandlerOnce.Do(func() {
		uh = &userHandler{
			onlineUserMap: &sync.Map{},
			handlerMap: map[common.MessageCode]common.Handler{
				enum.UserLogin:         &loginHandler{},
				enum.GetOnlineUserList: &getOnlineUserListHandler{},
				enum.UserLogout:        &logoutHandler{},
				enum.SendMessage:       &sendMessageHandler{},
			},
		}
	})
	return uh
}

func (h *userHandler) OnMessage(_ common.Context, _ *common.RawMessage) error {
	return nil
}

func (h *userHandler) OnActive(ctx common.Context) {}

func (h *userHandler) OnClose(ctx common.Context) {}

func (h *userHandler) OnInit(env common.Env) {
	for code, handler := range h.handlerMap {
		env.AddHandler(code, handler)
	}
}

func (h *userHandler) OnRemove(env common.Env) {
	for _, handler := range h.handlerMap {
		handler.OnRemove(env)
	}
}

func (h *userHandler) AddOnlineUser(user *OnlineUser) {
	h.onlineUserMap.Store(user.Addr(), user)
}

func (h *userHandler) RemoveOnlineUser(addr string) {
	_, ok := h.GetOnlineUser(addr)
	if ok {
		h.onlineUserMap.Delete(addr)
	}
}

func (h *userHandler) BroadcastMessage(targetUser []*OnlineUser, message *common.Message) {
	if len(targetUser) != 0 {
		for _, user := range targetUser {
			onlineUser, ok := h.GetOnlineUser(user.Addr())
			if ok {
				_ = onlineUser.ctx.Write(message)
			}
		}
		return
	}
	users := h.GetOnlineUsers(1000)
	for i := range users {
		err := users[i].ctx.Write(message)
		if err != nil {
			h.RemoveOnlineUser(users[i].Addr())
			_ = users[i].ctx.Close()
		}
	}
}

func (h *userHandler) GetOnlineUser(addr string) (*OnlineUser, bool) {
	user, ok := h.onlineUserMap.Load(addr)
	if !ok {
		return nil, ok
	}
	return user.(*OnlineUser), ok
}

func (h *userHandler) GetOnlineUsers(limit int) []*OnlineUser {
	users := make([]*OnlineUser, 0)
	h.onlineUserMap.Range(func(key, value interface{}) bool {
		if len(users) > limit {
			return false
		}
		user := value.(*OnlineUser)
		users = append(users, user)
		return true
	})
	return users
}

type loginHandler struct {
	common.BaseHandler
}

func (h *loginHandler) OnMessage(ctx common.Context, rawMessage *common.RawMessage) error {
	message := &msg.LoginMsg{}
	err := json.Unmarshal(rawMessage.RawData, message)
	if err != nil {
		log.Println(err)
		GetUserHandler().RemoveOnlineUser(ctx.RemoteAddr())
		_ = ctx.Write(goserver.NewDisplayMessage("invalid data"))
		_ = ctx.Close()
		return err
	}
	if user, ok := GetUserHandler().GetOnlineUser(ctx.RemoteAddr()); ok {
		err = ctx.Write(goserver.NewDisplayMessage("your already logged"))
		if err != nil {
			log.Println(err)
			GetUserHandler().RemoveOnlineUser(user.Addr())
			_ = ctx.Close()
		}
		return err
	}
	user := &OnlineUser{
		ctx: ctx,
		user: &msg.User{
			NickName: message.NickName,
		},
		addr: ctx.RemoteAddr(),
	}
	GetUserHandler().AddOnlineUser(user)
	loginMsg := fmt.Sprintf("login success, now %s, your address is %s", time.Now().String(), user.Addr())
	err = ctx.Write(goserver.NewDisplayMessage(loginMsg))
	if err != nil {
		log.Println(err)
		GetUserHandler().RemoveOnlineUser(user.Addr())
		_ = ctx.Close()
	}
	go GetUserHandler().BroadcastMessage(nil, goserver.NewDisplayMessage(user.NikeName()+"上线了"))
	return nil
}

func (h *loginHandler) OnActive(ctx common.Context) {
	_ = ctx.Write(goserver.NewDisplayMessage("hello, please login into the chat server"))
}

func (h *loginHandler) OnClose(ctx common.Context) {
	_, ok := GetUserHandler().GetOnlineUser(ctx.RemoteAddr())
	if !ok {
		return
	}
	GetUserHandler().RemoveOnlineUser(ctx.RemoteAddr())
}

type logoutHandler struct {
	common.BaseHandler
}

func (h *logoutHandler) OnMessage(ctx common.Context, _ *common.RawMessage) error {
	user, ok := GetUserHandler().GetOnlineUser(ctx.RemoteAddr())
	if !ok {
		return nil
	}
	GetUserHandler().RemoveOnlineUser(ctx.RemoteAddr())
	_ = ctx.Write(goserver.NewDisplayMessage("logout success"))
	go GetUserHandler().BroadcastMessage(nil, goserver.NewDisplayMessage(user.NikeName()+"离开了"))
	return nil
}

type getOnlineUserListHandler struct {
	common.BaseHandler
}

func (h *getOnlineUserListHandler) OnMessage(ctx common.Context, _ *common.RawMessage) error {
	users := GetUserHandler().GetOnlineUsers(1000)
	builder := &strings.Builder{}
	builder.WriteString(fmt.Sprintf("online user number: %d\n", len(users)))
	for i := range users {
		builder.WriteString(fmt.Sprintf("address=%s, nickname=%s\n", users[i].Addr(), users[i].NikeName()))
	}
	err := ctx.Write(goserver.NewDisplayMessage(builder.String()))
	if err != nil {
		log.Println(err)
		_ = ctx.Close()
		return err
	}
	return nil
}

type sendMessageHandler struct {
	common.BaseHandler
}

func (h *sendMessageHandler) OnMessage(ctx common.Context, msg *common.RawMessage) error {
	user, ok := GetUserHandler().GetOnlineUser(ctx.RemoteAddr())
	if !ok {
		err := ctx.Write(goserver.NewDisplayMessage("please login"))
		if err != nil {
			log.Println(err)
			_ = ctx.Close()
			return err
		}
	}
	str := ""
	err := json.Unmarshal(msg.RawData, &str)
	if err != nil {
		log.Println(err)
		return err
	}
	go GetUserHandler().BroadcastMessage(nil,
		goserver.NewDisplayMessage(user.NikeName()+",IP:"+user.Addr()+"\n\t"+str))
	return nil
}
