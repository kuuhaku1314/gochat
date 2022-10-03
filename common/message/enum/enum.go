package enum

import "gochat/common"

const (
	Echo common.MessageCode = iota + 1
	UserLogin
	UserLogout
	GetOnlineUserList
	GetRemoteRealIPInfo
	Ping
	Pong
)
