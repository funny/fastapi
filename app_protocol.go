package fastapi

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/funny/link"
	"github.com/funny/slab"
)

func (app *App) newClientCodec(rw io.ReadWriter) (link.Codec, error) {
	return app.newCodec(rw, app.newResponse), nil
}

func (app *App) newServerCodec(rw io.ReadWriter) (link.Codec, error) {
	return app.newCodec(rw, app.newRequest), nil
}

func (app *App) newCodec(rw io.ReadWriter, newMessage func(byte, byte) (Message, error)) link.Codec {
	c := &codec{
		conn:        rw.(net.Conn),
		reader:      bufio.NewReaderSize(rw, app.ReadBufSize),
		pool:        app.Pool,
		maxRecvSize: app.MaxRecvSize,
		maxSendSize: app.MaxSendSize,
		newMessage:  newMessage,
	}
	c.headBuf = c.headData[:]
	return c
}

func (app *App) newRequest(serviceID, messageID byte) (Message, error) {
	if service := app.services[serviceID]; service != nil {
		if msg := service.(Service).NewRequest(messageID); msg != nil {
			return msg, nil
		}
		return nil, DecodeError{fmt.Sprintf("Unsupported Message Type: [%d, %d]", serviceID, messageID)}
	}
	return nil, DecodeError{fmt.Sprintf("Unsupported Service: [%d, %d]", serviceID, messageID)}
}

func (app *App) newResponse(serviceID, messageID byte) (Message, error) {
	if service := app.services[serviceID]; service != nil {
		if msg := service.(Service).NewResponse(messageID); msg != nil {
			return msg, nil
		}
		return nil, DecodeError{fmt.Sprintf("Unsupported Message Type: [%d, %d]", serviceID, messageID)}
	}
	return nil, DecodeError{fmt.Sprintf("Unsupported Service: [%d, %d]", serviceID, messageID)}
}

const packetHeadSize = 4 + 2

type codec struct {
	headBuf     []byte
	headData    [packetHeadSize]byte
	conn        net.Conn
	reader      *bufio.Reader
	pool        slab.Pool
	maxRecvSize int
	maxSendSize int
	newMessage  func(byte, byte) (Message, error)
}

func (c *codec) Conn() net.Conn {
	return c.conn
}

func (c *codec) Receive() (msg interface{}, err error) {
	if _, err = io.ReadFull(c.reader, c.headBuf); err != nil {
		return
	}

	packetSize := int(binary.LittleEndian.Uint32(c.headBuf))

	if packetSize > c.maxRecvSize {
		return nil, DecodeError{fmt.Sprintf("Too Large Receive Packet Size: %d", packetSize)}
	}

	packet := c.pool.Alloc(packetSize)

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

	c.pool.Free(packet)
	return
}

func (c *codec) Send(m interface{}) (err error) {
	msg := m.(Message)

	packetSize := msg.BinarySize()

	if packetSize > c.maxSendSize {
		panic(EncodeError{fmt.Sprintf("Too Large Send Packet Size: %d", packetSize)})
	}

	packet := c.pool.Alloc(packetHeadSize + packetSize)
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
	c.pool.Free(packet)
	return
}

func (c *codec) Close() error {
	return c.conn.Close()
}

type msgFormat struct {
	newMessage func(byte, byte) (Message, error)
}

func (f *msgFormat) EncodeMessage(msg interface{}) ([]byte, error) {
	msg2 := msg.(Message)
	buf := make([]byte, 2+msg2.BinarySize())
	buf[0] = msg2.ServiceID()
	buf[1] = msg2.MessageID()
	msg2.MarshalPacket(buf[2:])
	return buf, nil
}

func (f *msgFormat) DecodeMessage(msg []byte) (interface{}, error) {
	msg2, err := f.newMessage(msg[0], msg[1])
	if err != nil {
		return nil, err
	}
	msg2.UnmarshalPacket(msg[2:])
	return msg2, nil
}
