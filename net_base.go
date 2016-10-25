package fastapi

import (
	"fmt"

	"github.com/funny/link"
)

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
	Allocator    Allocator
	ReadBufSize  int
	SendChanSize int
	MaxRecvSize  int
	MaxSendSize  int
}

// Allocator provide a way to pooling memory.
// Reference: https://github.com/funny/slab
type Allocator interface {
	Alloc(int) []byte
	Free([]byte)
}

type nonAllocator struct{}

func (_ nonAllocator) Alloc(size int) []byte {
	return make([]byte, size)
}

func (_ nonAllocator) Free(_ []byte) {}

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
