package chatclient

import (
	"encoding/json"
	"gochat/common"
	"gochat/common/message/enum"
)

func GetOnlineUserHandler(ctx ClientContext, _ json.RawMessage) error {
	return ctx.Write(&common.Message{
		Code:    enum.GetOnlineUserList,
		RawData: nil,
	})
}
