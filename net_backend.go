package fastapi

import (
	"fmt"
	"reflect"

	fastway "github.com/funny/fastway/go"
	"github.com/funny/link"
)

type msgFormat struct {
	newMessage func(byte, byte) (Message, error)
}

func (f *msgFormat) EncodeMessage(msg interface{}) ([]byte, error) {
	msg2 := msg.(Message)
	buf := make([]byte, msg2.BinarySize())
	msg2.MarshalPacket(buf)
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

type ConnHandler interface {
	HandleConn(*fastway.Conn)
}

type ConnHandlerFunc func(*fastway.Conn)

func (f ConnHandlerFunc) HandleConn(conn *fastway.Conn) {
	f(conn)
}

var _ IServer = (*Backend)(nil)

type Backend struct {
	endpoint *fastway.EndPoint
	services [256]Service
}

func (app *App) DialBackend(network, addr string, cfg fastway.EndPointCfg) (backend *Backend, err error) {
	backend = new(Backend)

	cfg.MsgFormat = &msgFormat{
		newMessage: backend.newMsg,
	}

	backend.endpoint, err = fastway.DialServer(network, addr, cfg)
	if err != nil {
		return nil, err
	}

	for i, s := range app.services {
		if s != nil {
			backend.services[i] = reflect.New(s).Interface().(Service)
		}
	}
	return
}

func (backend *Backend) Stop() {
	backend.endpoint.Close()
}

func (backend *Backend) Serve(handler link.Handler) error {
	defer backend.endpoint.Close()
	for {
		conn, err := backend.endpoint.Accept()
		if err != nil {
			return err
		}
		go handler.HandleSession(conn.Session)
	}
	return nil
}

func (backend *Backend) ServeConn(handler ConnHandler) error {
	defer backend.endpoint.Close()
	for {
		conn, err := backend.endpoint.Accept()
		if err != nil {
			return err
		}
		go handler.HandleConn(conn)
	}
	return nil
}

func (backend *Backend) GetSession(sessionID uint64) *link.Session {
	return backend.endpoint.GetSession(sessionID)
}

func (backend *Backend) Init(initializer func(Service)) {
	for _, s := range backend.services {
		if s != nil {
			initializer(s)
		}
	}
}

func (backend *Backend) Dispatch(session *link.Session, req Message) {
	backend.services[req.ServiceID()].HandleRequest(session, req)
}

func (backend *Backend) newMsg(serviceID, messageID byte) (Message, error) {
	if service := backend.services[serviceID]; service != nil {
		if msg := service.NewRequest(messageID); msg != nil {
			return msg, nil
		}
		return nil, DecodeError{fmt.Sprintf("Unsupported Message Type: [%d, %d]", serviceID, messageID)}
	}
	return nil, DecodeError{fmt.Sprintf("Unsupported Service: [%d, %d]", serviceID, messageID)}
}
