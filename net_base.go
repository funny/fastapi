package fastapi

import (
	"fmt"

	"github.com/funny/link"
	"github.com/funny/slab"
)

type IServer interface {
	Init(initializer func(Service))
	Serve(link.Handler) error
	Dispatch(*link.Session, Message)
	Stop()
	GetSession(uint64) *link.Session
}

type Service interface {
	ServiceID() byte
	NewRequest(byte) Message
	NewResponse(byte) Message
	HandleRequest(*link.Session, Message)
}

type Message interface {
	ServiceID() byte
	MessageID() byte
	Identity() string
	BinarySize() int
	MarshalPacket([]byte)
	UnmarshalPacket([]byte)
}

type Config struct {
	Pool         slab.Pool
	ReadBufSize  int
	SendChanSize int
	MaxRecvSize  int
	MaxSendSize  int
}

type EncodeError struct {
	Message interface{}
}

func (encodeError EncodeError) Error() string {
	return fmt.Sprintf("Encode Error: %v", encodeError.Message)
}

type DecodeError struct {
	Message interface{}
}

func (decodeError DecodeError) Error() string {
	return fmt.Sprintf("Decode Error: %v", decodeError.Message)
}
