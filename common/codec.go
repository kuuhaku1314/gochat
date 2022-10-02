package common

import (
	"encoding/json"
	"errors"
	"io"
)

const (
	JsonCodecType = 1
	GobCodecType  = 2
)

type Codec interface {
	Encode(interface{}) error
	Decode(interface{}) error
}

type JsonCodec struct {
	*json.Decoder
	*json.Encoder
}

func NewJsonCodec(readWriter io.ReadWriter) Codec {
	return &JsonCodec{
		Decoder: json.NewDecoder(readWriter),
		Encoder: json.NewEncoder(readWriter),
	}
}

func GetCodec(codecType int8, reader io.ReadWriter) (Codec, error)  {
	switch codecType {
	case JsonCodecType:
		return NewJsonCodec(reader), nil
	default:
		return nil, errors.New("invalid codec type")
	}
}