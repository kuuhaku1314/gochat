package handler

import (
	"encoding/json"
	"fmt"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/message/msg"
	"gochat/common/util"
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
				enum.FileTransfer:      &fileTransferHandler{},
			},
		}
	})
	return uh
}

func (h *userHandler) OnMessage(_ common.Context, _ *common.RawMessage) error {
	return nil
}

func (h *userHandler) OnActive(ctx common.Context) {
	_ = ctx.Write(util.NewDisplayMessage("hello, please login"))
}

func (h *userHandler) OnClose(_ common.Context) {}

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

func (h *userHandler) CheckLogin(ctx common.Context) (*OnlineUser, bool) {
	user, ok := h.GetOnlineUser(ctx.RemoteAddr())
	if !ok {
		err := ctx.Write(util.NewDisplayMessage("please login"))
		if err != nil {
			log.Println(err)
			_ = ctx.Close()
		}
		return nil, false
	}
	return user, true
}

type loginHandler struct {
	common.BaseHandler
}

func (h *loginHandler) OnMessage(ctx common.Context, rawMessage *common.RawMessage) error {
	message := &msg.LoginMsg{}
	if err := json.Unmarshal(rawMessage.RawData, message); err != nil {
		GetUserHandler().RemoveOnlineUser(ctx.RemoteAddr())
		_ = ctx.Write(util.NewDisplayMessage("invalid data"))
		_ = ctx.Close()
		return err
	}
	if user, ok := GetUserHandler().GetOnlineUser(ctx.RemoteAddr()); ok {
		err := ctx.Write(util.NewDisplayMessage("your already logged"))
		if err != nil {
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
	if err := ctx.Write(util.NewDisplayMessage(loginMsg)); err != nil {
		GetUserHandler().RemoveOnlineUser(user.Addr())
		_ = ctx.Close()
		return err
	}
	go GetUserHandler().BroadcastMessage(nil, util.NewDisplayMessage(user.NikeName()+"上线了"))
	return nil
}

func (h *loginHandler) OnActive(_ common.Context) {}

func (h *loginHandler) OnClose(ctx common.Context) {
	user, ok := GetUserHandler().GetOnlineUser(ctx.RemoteAddr())
	if !ok {
		return
	}
	GetUserHandler().RemoveOnlineUser(ctx.RemoteAddr())
	GetUserHandler().BroadcastMessage(nil, util.NewDisplayMessage(user.NikeName()+"掉线了"))
}

type logoutHandler struct {
	common.BaseHandler
}

func (h *logoutHandler) OnMessage(ctx common.Context, _ *common.RawMessage) error {
	user, ok := GetUserHandler().CheckLogin(ctx)
	if !ok {
		return nil
	}
	GetUserHandler().RemoveOnlineUser(ctx.RemoteAddr())
	_ = ctx.Write(util.NewDisplayMessage("logout success"))
	go GetUserHandler().BroadcastMessage(nil, util.NewDisplayMessage(user.NikeName()+"离开了"))
	return nil
}

type getOnlineUserListHandler struct {
	common.BaseHandler
}

func (h *getOnlineUserListHandler) OnMessage(ctx common.Context, _ *common.RawMessage) error {
	_, ok := GetUserHandler().CheckLogin(ctx)
	if !ok {
		return nil
	}
	users := GetUserHandler().GetOnlineUsers(1000)
	builder := &strings.Builder{}
	builder.WriteString(fmt.Sprintf("online user number: %d\n", len(users)))
	for i := range users {
		builder.WriteString(fmt.Sprintf("address=%s, nickname=%s\n", users[i].Addr(), users[i].NikeName()))
	}
	err := ctx.Write(util.NewDisplayMessage(builder.String()))
	if err != nil {
		_ = ctx.Close()
		return err
	}
	return nil
}

type sendMessageHandler struct {
	common.BaseHandler
}

func (h *sendMessageHandler) OnMessage(ctx common.Context, msg *common.RawMessage) error {
	user, ok := GetUserHandler().CheckLogin(ctx)
	if !ok {
		return nil
	}
	str := ""
	if err := json.Unmarshal(msg.RawData, &str); err != nil {
		return err
	}
	go GetUserHandler().BroadcastMessage(nil,
		util.NewDisplayMessage(user.NikeName()+",IP:"+user.Addr()+"\n\t"+str))
	return nil
}

type fileTransferHandler struct {
	common.BaseHandler
}

func (h *fileTransferHandler) OnMessage(ctx common.Context, rawMessage *common.RawMessage) error {
	_, ok := GetUserHandler().CheckLogin(ctx)
	if !ok {
		return nil
	}
	transformEntity := &msg.FileTransformEntity{}
	if err := json.Unmarshal(rawMessage.RawData, &transformEntity); err != nil {
		return err
	}
	if ctx.RemoteAddr() != transformEntity.From {
		return ctx.Write(util.NewDisplayMessage("dont send fake message, your ip is " + ctx.RemoteAddr()))
	}
	receiver, ok := GetUserHandler().GetOnlineUser(transformEntity.To)
	if !ok {
		return ctx.Write(util.NewDisplayMessage("not found receiver"))
	}
	GetUserHandler().BroadcastMessage([]*OnlineUser{receiver},
		&common.Message{
			Code:    enum.FileTransfer,
			RawData: transformEntity,
		})
	return nil
}
