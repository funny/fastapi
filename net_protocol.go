package fastapi

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/funny/link"
)

type protocol struct {
	config     *Config
	newMessage func(byte, byte) (Message, error)
}

func (p *protocol) NewCodec(rw io.ReadWriter) (link.Codec, error) {
	c := &Codec{
		conn:        rw.(net.Conn),
		reader:      bufio.NewReaderSize(rw, p.config.ReadBufSize),
		allocator:   p.config.Allocator,
		maxRecvSize: p.config.MaxRecvSize,
		maxSendSize: p.config.MaxSendSize,
		newMessage:  p.newMessage,
	}
	c.headBuf = c.headData[:]
	return c, nil
}

const packetHeadSize = 4 + 2

type Codec struct {
	headBuf     []byte
	headData    [packetHeadSize]byte
	conn        net.Conn
	reader      *bufio.Reader
	allocator   Allocator
	maxRecvSize int
	maxSendSize int
	newMessage  func(byte, byte) (Message, error)
}

func (c *Codec) Conn() net.Conn {
	return c.conn
}

func (c *Codec) Receive() (msg interface{}, err error) {
	if _, err = io.ReadFull(c.reader, c.headBuf); err != nil {
		return
	}

	packetSize := int(binary.LittleEndian.Uint32(c.headBuf))

	if packetSize > c.maxRecvSize {
		return nil, DecodeError{fmt.Sprintf("Too Large Receive Packet Size: %d", packetSize)}
	}

	packet := c.allocator.Alloc(packetSize)

	if _, err = io.ReadFull(c.reader, packet); err == nil {
		msg1, err1 := c.newMessage(c.headData[4], c.headData[5])
		if err1 == nil {
			func() {
				defer func() {
					if panicErr := recover(); panicErr != nil {
						err = DecodeError{panicErr}
					}
				}()
				msg1.UnmarshalPacket(packet)
			}()
			msg = msg1
		} else {
			err = err1
		}
	}

	c.allocator.Free(packet)
	return
}

func (c *Codec) Send(m interface{}) (err error) {
	msg := m.(Message)

	packetSize := msg.BinarySize()

	if packetSize > c.maxSendSize {
		panic(EncodeError{fmt.Sprintf("Too Large Send Packet Size: %d", packetSize)})
	}

	packet := c.allocator.Alloc(packetHeadSize + packetSize)
	binary.LittleEndian.PutUint32(packet, uint32(packetSize))
	packet[4] = msg.ServiceID()
	packet[5] = msg.MessageID()

	func() {
		defer func() {
			if err := recover(); err != nil {
				err = EncodeError{err}
			}
		}()
		msg.MarshalPacket(packet[packetHeadSize:])
	}()

	_, err = c.conn.Write(packet)
	c.allocator.Free(packet)
	return
}

func (c *Codec) Close() error {
	return c.conn.Close()
}
