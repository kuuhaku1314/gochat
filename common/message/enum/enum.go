package enum

import "gochat/common"

const (
	Display common.MessageCode = iota + 1
	UserLogin
	UserLogout
	GetOnlineUserList
	GetRemoteRealIPInfo
	Ping
	Pong
)
