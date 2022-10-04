package main

import (
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/message/msg"
	"gochat/goclient"
)

func NewSendCommand() *goclient.Command {
	return &goclient.Command{
		Command: "send",
		Alias:   nil,
		ParseFunc: func(params string) (*common.Message, error) {
			return &common.Message{
				Code:    enum.SendMessage,
				RawData: params,
			}, nil
		},
		UseParseFunc:   true,
		LocalParseFunc: nil,
		Tips:           "send message to everyone",
	}
}

func NewLoginCommand() *goclient.Command {
	return &goclient.Command{
		Command: "login",
		Alias:   nil,
		ParseFunc: func(params string) (*common.Message, error) {
			return &common.Message{
				Code:    enum.UserLogin,
				RawData: &msg.LoginMsg{NickName: params},
			}, nil
		},
		UseParseFunc:   true,
		LocalParseFunc: nil,
		Tips:           "use like login [nickname]",
	}
}

func NewLogoutCommand() *goclient.Command {
	return &goclient.Command{
		Command: "logout",
		Alias:   nil,
		ParseFunc: func(params string) (*common.Message, error) {
			return &common.Message{
				Code:    enum.UserLogout,
				RawData: nil,
			}, nil
		},
		UseParseFunc:   true,
		LocalParseFunc: nil,
		Tips:           "use like logout",
	}
}

func NewGetUserListCommand() *goclient.Command {
	return &goclient.Command{
		Command: "userlist",
		Alias:   nil,
		ParseFunc: func(params string) (*common.Message, error) {
			return &common.Message{
				Code:    enum.GetOnlineUserList,
				RawData: nil,
			}, nil
		},
		UseParseFunc:   true,
		LocalParseFunc: nil,
		Tips:           "use like userlist",
	}
}
