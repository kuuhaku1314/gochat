package main

import (
	"encoding/base64"
	"encoding/json"
	"gochat/common"
	"gochat/common/message/enum"
	"gochat/common/message/msg"
	"gochat/common/util"
	"gochat/goclient"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

type fileTransferHandler struct {
	sendFileEntity      *msg.FileTransformEntity
	sendLock            *sync.Mutex
	sendFile            *os.File
	lastSendFileTime    int64
	sendBlock           int64
	sendBuff            []byte
	receiveFileEntity   *msg.FileTransformEntity
	receiveLock         *sync.Mutex
	receiveFile         *os.File
	receiveBlock        int64
	lastReceiveFileTime int64
	msgHandler          map[int8]func(ctx common.Context, file *msg.FileTransformEntity) error
	timeout             int64
	client              *goclient.Client
}

func NewFileTransferHandler(client *goclient.Client, timeout time.Duration) *fileTransferHandler {
	fileTransfer := &fileTransferHandler{
		sendBuff:    make([]byte, 1024*64),
		sendLock:    &sync.Mutex{},
		receiveLock: &sync.Mutex{},
		timeout:     int64(timeout.Seconds()),
		client:      client,
	}
	fileTransfer.msgHandler = map[int8]func(ctx common.Context, file *msg.FileTransformEntity) error{
		msg.FileWaitingSend:   fileTransfer.FileStateWaitingSend,
		msg.FileReject:        fileTransfer.FileStateReject,
		msg.FileSending:       fileTransfer.FileStateSending,
		msg.FileAck:           fileTransfer.FileStateAck,
		msg.FileSendCompleted: fileTransfer.FileStateCompleted,
	}
	return fileTransfer
}

func (h *fileTransferHandler) commands() []*goclient.Command {
	return []*goclient.Command{h.confirmAccept(), h.rejectAccept(), h.trySendFile()}
}

func (h *fileTransferHandler) trySendFile() *goclient.Command {
	return &goclient.Command{
		Command:      "sendfile",
		UseParseFunc: false,
		LocalParseFunc: func(params string) error {
			arr := strings.SplitN(params, " ", 3)
			if len(arr) < 3 {
				log.Println("invalid params")
				return nil
			}
			localID, remoteID, path := strings.TrimSpace(arr[0]), strings.TrimSpace(arr[1]), strings.TrimSpace(arr[2])
			if len(localID) == 0 || len(remoteID) == 0 || len(path) == 0 {
				log.Println("invalid params")
				return nil
			}
			if err := h.notifySendFile(localID, remoteID, path); err != nil {
				log.Println(err)
			}
			return nil
		},
		Tips: "use like: sendfile [localID] [remoteID] [path]",
	}
}

func (h *fileTransferHandler) confirmAccept() *goclient.Command {
	return &goclient.Command{
		Command: "confirm",
		ParseFunc: func(params string) (*common.Message, error) {
			h.receiveLock.Lock()
			defer h.receiveLock.Unlock()
			if h.receiveFileEntity == nil {
				log.Println("not found receiveFileEntity")
				return nil, nil
			}
			if h.receiveFileEntity.State == msg.FileWaitingSend {
				_, err := os.Stat(params)
				if err != nil && !os.IsNotExist(err) {
					log.Println(err)
					return nil, nil
				}
				if os.IsExist(err) {
					log.Println("该文件已存在，请换个文件名")
					return nil, nil
				}
				if strings.HasSuffix(params, string(os.PathSeparator)) {
					log.Println("不能使用目录当文件名")
					return nil, nil
				}
				log.Println("开始接受文件")
				file, err := os.Create(params)
				if err != nil {
					log.Printf("error=%s, please retry", err)
					return nil, nil
				}
				h.receiveFile = file
			}
			h.receiveFileEntity.State = msg.FileAccept
			return &common.Message{
				Code: enum.FileTransfer,
				RawData: &msg.FileTransformEntity{
					To:    h.receiveFileEntity.From,
					From:  h.receiveFileEntity.To,
					State: msg.FileAck,
				},
			}, nil
		},
		UseParseFunc: true,
		Tips:         "use confirm to receive file, use like: confirm [path]",
	}
}

func (h *fileTransferHandler) rejectAccept() *goclient.Command {
	return &goclient.Command{
		Command: "reject",
		ParseFunc: func(_ string) (*common.Message, error) {
			h.receiveLock.Lock()
			defer h.receiveLock.Unlock()
			if h.receiveFileEntity == nil {
				log.Println("not found receiveFileEntity")
				return nil, nil
			}
			if h.receiveFileEntity.State != msg.FileWaitingSend {
				return nil, nil
			}
			message := &common.Message{
				Code: enum.FileTransfer,
				RawData: &msg.FileTransformEntity{
					To:    h.receiveFileEntity.From,
					From:  h.receiveFileEntity.To,
					State: msg.FileReject,
				},
			}
			h.receiveFileEntity = nil
			return message, nil
		},
		UseParseFunc: true,
		Tips:         "use reject to reject receive file",
	}
}

func (h *fileTransferHandler) FileStateAck(ctx common.Context, fileTransformEntity *msg.FileTransformEntity) error {
	h.sendLock.Lock()
	defer h.sendLock.Unlock()
	if !h.checkSend(fileTransformEntity, -1) {
		return nil
	}
	n, err := h.sendFile.Read(h.sendBuff)
	if err != nil && err != io.EOF {
		return err
	}
	eofFlag := err == io.EOF || n < len(h.sendBuff)
	h.sendFileEntity.Content = base64.StdEncoding.EncodeToString(h.sendBuff[:n])
	if eofFlag {
		h.sendFileEntity.State = msg.FileSendCompleted
	} else {
		h.sendFileEntity.State = msg.FileSending
	}
	err = ctx.Write(&common.Message{
		Code:    enum.FileTransfer,
		RawData: h.sendFileEntity,
	})
	h.lastSendFileTime = time.Now().Unix()
	h.sendBlock++
	log.Printf("send file blocksize=[%d], [%d/%d]\n", len(h.sendFileEntity.Content), h.sendBlock,
		int64(math.Round(float64(h.sendFileEntity.FileSize)/float64(len(h.sendBuff)))))
	if eofFlag {
		log.Println("send file complete")
		h.resetSendFile(true)
	}
	return err
}

func (h *fileTransferHandler) FileStateReject(_ common.Context, fileTransformEntity *msg.FileTransformEntity) error {
	h.sendLock.Lock()
	defer h.sendLock.Unlock()
	if !h.checkSend(fileTransformEntity, msg.FileWaitingSend) {
		return nil
	}
	h.resetSendFile(true)
	log.Println("对方拒绝了你的请求")
	return nil
}

func (h *fileTransferHandler) FileStateWaitingSend(ctx common.Context, fileTransformEntity *msg.FileTransformEntity) error {
	h.receiveLock.Lock()
	defer h.receiveLock.Unlock()
	entity := h.receiveFileEntity
	if entity != nil {
		log.Printf("ID:%s 想要给你发送文件，文件名:%s, 文件大小:%db，但正在接受文件中所以自动拒绝", fileTransformEntity.From,
			fileTransformEntity.FileName, fileTransformEntity.FileSize)
		return ctx.Write(&common.Message{
			Code: enum.FileTransfer,
			RawData: &msg.FileTransformEntity{
				To:    fileTransformEntity.From,
				From:  fileTransformEntity.To,
				State: msg.FileReject,
			},
		})
	}
	h.receiveFileEntity = fileTransformEntity
	log.Printf("ID%s 想要给你发送文件，文件名:%s, 文件大小:%db", fileTransformEntity.From,
		fileTransformEntity.FileName, fileTransformEntity.FileSize)
	log.Printf("请回复confirm [filepath]去接收或reject拒绝接收")
	return nil
}

func (h *fileTransferHandler) FileStateSending(ctx common.Context, fileTransformEntity *msg.FileTransformEntity) error {
	h.receiveLock.Lock()
	defer h.receiveLock.Unlock()
	if !h.validateReceiveFile(fileTransformEntity, msg.FileAccept) {
		return nil
	}
	h.lastReceiveFileTime = time.Now().Unix()
	h.receiveBlock++
	bytes, err := base64.StdEncoding.DecodeString(fileTransformEntity.Content)
	if err != nil {
		return err
	}
	log.Printf("receiving file blocksize=[%d], [%d/%d]\n", len(fileTransformEntity.Content), h.receiveBlock,
		int64(math.Ceil(float64(h.receiveFileEntity.FileSize)/float64(len(h.sendBuff)))))
	if _, err := h.receiveFile.Write(bytes); err != nil {
		return err
	}
	return ctx.Write(&common.Message{
		Code: enum.FileTransfer,
		RawData: &msg.FileTransformEntity{
			To:    h.receiveFileEntity.From,
			From:  h.receiveFileEntity.To,
			State: msg.FileAck,
		},
	})
}

func (h *fileTransferHandler) FileStateCompleted(_ common.Context, fileTransformEntity *msg.FileTransformEntity) error {
	h.receiveLock.Lock()
	defer h.receiveLock.Unlock()
	if !h.validateReceiveFile(fileTransformEntity, msg.FileAccept) {
		return nil
	}
	h.lastReceiveFileTime = time.Now().Unix()
	h.receiveBlock++
	bytes, err := base64.StdEncoding.DecodeString(fileTransformEntity.Content)
	if err != nil {
		return err
	}
	log.Printf("receiving file blocksize=[%d], [%d/%d]\n", len(fileTransformEntity.Content), h.receiveBlock,
		int64(math.Ceil(float64(h.receiveFileEntity.FileSize)/float64(len(h.sendBuff)))))
	if _, err := h.receiveFile.Write(bytes); err != nil {
		return err
	}
	log.Printf("receive file completed, local filename=%s", h.receiveFile.Name())
	h.resetReceiveFile(true)
	return nil
}

func (h *fileTransferHandler) OnMessage(ctx common.Context, rawMessage *common.RawMessage) error {
	message := &msg.FileTransformEntity{}
	if err := json.Unmarshal(rawMessage.RawData, message); err != nil {
		return err
	}
	f, ok := h.msgHandler[message.State]
	if !ok {
		log.Println("invalid state")
		return nil
	}
	return f(ctx, message)
}

func (h *fileTransferHandler) OnActive(_ common.Context) {}

func (h *fileTransferHandler) OnClose(_ common.Context) {}

func (h *fileTransferHandler) OnInit(_ common.Env) {
	for _, command := range h.commands() {
		util.AssertNotError(h.client.Register(command))
	}
	ticker := time.NewTicker(time.Second * 5)
	go func() {
		log.Println("[start file watch]")
		for {
			<-ticker.C
			if !h.checkSendFileTimeout() {
				log.Println("checkSendFileTimeout false")
				h.sendLock.Lock()
				if !h.checkSendFileTimeout() {
					h.resetSendFile(false)
				}
				h.sendLock.Unlock()
			}

			if !h.checkReceiveFileTimeout() {
				log.Println("checkReceiveFileTimeout false")
				h.receiveLock.Lock()
				if !h.checkReceiveFileTimeout() {
					h.resetReceiveFile(false)
				}
				h.receiveLock.Unlock()
			}
		}
	}()
}

func (h *fileTransferHandler) OnRemove(_ common.Env) {}

func (h *fileTransferHandler) checkReceiveFileTimeout() bool {
	if h.receiveFileEntity != nil && h.receiveFile != nil && h.lastReceiveFileTime != 0 {
		return h.lastReceiveFileTime+h.timeout > time.Now().Unix()
	}
	return true
}

func (h *fileTransferHandler) checkSendFileTimeout() bool {
	if h.sendFileEntity != nil && h.sendFile != nil && h.lastSendFileTime != 0 {
		return h.lastSendFileTime+h.timeout > time.Now().Unix()
	}
	return true
}

func (h *fileTransferHandler) notifySendFile(localID, remoteID, filepath string) error {
	h.sendLock.Lock()
	defer h.sendLock.Unlock()
	if h.sendFileEntity != nil {
		log.Printf("当前存在发送中文件，请等待发送完成, 若正在等待对方确认中，请等待%d秒后自动取消发送\n",
			h.lastSendFileTime+h.timeout-time.Now().Unix())
		return nil
	}
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		log.Println("不支持文件夹传输")
		return nil
	}
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	h.sendFile = file
	h.sendFileEntity = &msg.FileTransformEntity{
		FileSize: fileInfo.Size(),
		FileName: fileInfo.Name(),
		To:       remoteID,
		From:     localID,
		Content:  "",
		State:    msg.FileWaitingSend,
	}
	h.lastSendFileTime = time.Now().Unix()
	h.client.SendMessage(&common.Message{
		Code:    enum.FileTransfer,
		RawData: h.sendFileEntity,
	})
	log.Printf("请求发送成功，正在等待对方接受或拒绝响应， 最多等待%d秒后自动取消发送\n", h.timeout)
	return nil
}

func (h *fileTransferHandler) resetReceiveFile(noneError bool) {
	if h.receiveFile != nil {
		name := h.receiveFile.Name()
		if err := h.receiveFile.Close(); err != nil {
			log.Println(err)
		}
		if !noneError {
			log.Printf("remove receive failed file:" + name)
			if err := os.Remove(name); err != nil {
				log.Println(err)
			}
		}
	}
	h.receiveFile = nil
	h.receiveFileEntity = nil
	h.lastReceiveFileTime = 0
	h.receiveBlock = 0
}

func (h *fileTransferHandler) resetSendFile(noneError bool) {
	if h.sendFile != nil {
		if err := h.sendFile.Close(); err != nil {
			log.Println(err)
		}
		if !noneError {
			log.Printf("send file failed, release fd")
		}
	}
	h.sendFile = nil
	h.sendFileEntity = nil
	h.lastSendFileTime = 0
	h.sendBlock = 0
}

func (h *fileTransferHandler) checkSend(fileTransformEntity *msg.FileTransformEntity, targetState int8) bool {
	if h.sendFileEntity == nil {
		log.Println("invalid file ack")
		return false
	}
	if h.sendFileEntity.From != fileTransformEntity.To || h.sendFileEntity.To != fileTransformEntity.From {
		log.Println("invalid file ack sender")
		return false
	}
	if targetState > 0 && h.sendFileEntity.State != targetState {
		log.Println("invalid file state")
		return false
	}
	return true
}

func (h *fileTransferHandler) validateReceiveFile(fileTransformEntity *msg.FileTransformEntity, targetState int8) bool {
	if h.receiveFileEntity == nil {
		log.Println("invalid sending request")
		return false
	}
	if h.receiveFileEntity.From != fileTransformEntity.From || h.receiveFileEntity.To != fileTransformEntity.To {
		log.Println("invalid sending sender")
		return false
	}
	if h.receiveFileEntity.FileSize != fileTransformEntity.FileSize ||
		h.receiveFileEntity.FileName != fileTransformEntity.FileName {
		log.Println("invalid file")
		return false
	}
	if targetState > 0 && h.receiveFileEntity.State != targetState {
		log.Println("invalid file state")
		return false
	}
	return true
}
