package msg

const (
	FileWaitingSend = iota + 1
	FileReject
	FileSending
	FileAccept
	FileAck
	FileSendCompleted
)

type FileTransformEntity struct {
	FileSize int64
	FileName string
	To       string
	From     string
	Content  string
	State    int8
}
