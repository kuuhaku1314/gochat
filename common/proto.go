package common

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
)

const MagicNumber int64 = 0x10086

type MessageCode int64
type CodecType int8

type Header struct {
	MagicNumber int64
	CodecType
}

func NewHeader(codecType CodecType) *Header {
	return &Header{
		MagicNumber: MagicNumber,
		CodecType:   codecType,
	}
}

func (h *Header) Validate() error {
	if h.MagicNumber != MagicNumber {
		return errors.New("invalid magic number")
	}
	return nil
}

func (h *Header) Bytes() []byte {
	number := h.MagicNumber
	bytes := make([]byte, 9)
	bytes[0] = (byte)(number)
	bytes[1] = (byte)(number >> 8)
	bytes[2] = (byte)(number >> 16)
	bytes[3] = (byte)(number >> 24)
	bytes[4] = (byte)(number >> 32)
	bytes[5] = (byte)(number >> 40)
	bytes[6] = (byte)(number >> 48)
	bytes[7] = (byte)(number >> 56)
	bytes[8] = (byte)(h.CodecType)
	return bytes
}

func ReadHeader(reader io.Reader) (*Header, error) {
	var (
		ctx, cancelFunc = context.WithTimeout(context.TODO(), time.Second*5)
		c               = make(chan struct{}, 1)
		bytes           = make([]byte, 9)
		err             error
		timeoutFlag     bool
	)

	go func() {
		_, err = reader.Read(bytes)
		c <- struct{}{}
	}()
	select {
	case <-c:
	case <-ctx.Done():
		timeoutFlag = true
	}
	cancelFunc()
	if timeoutFlag {
		return nil, errors.New("read header timeout")
	}
	if err != nil {
		return nil, err
	}
	magicNumber := int64(0)
	for i := 7; i >= 0; i-- {
		magicNumber <<= 8
		magicNumber = int64(bytes[i]) | magicNumber
	}
	return &Header{
		MagicNumber: magicNumber,
		CodecType:   CodecType(bytes[8]),
	}, nil
}

type RawMessage struct {
	Code    MessageCode     `json:"code"`
	RawData json.RawMessage `json:"raw_data"`
}

type Message struct {
	Code    MessageCode `json:"code"`
	RawData interface{} `json:"raw_data"`
}
