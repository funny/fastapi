package fastapi

import (
	"fmt"
	"reflect"

	fastway "github.com/funny/fastway/go"
	"github.com/funny/link"
)

type Backend struct {
	endpoint *fastway.EndPoint
	services [256]FullService
}

func (app *App) NewBackend(ep *fastway.EndPoint) *Backend {
	backend := new(Backend)
	backend.endpoint = ep
	for i, s := range app.services {
		if s != nil {
			backend.services[i] = reflect.New(s).Interface().(FullService)
		}
	}
	return backend
}

func (backend *Backend) Serve(handler link.Handler) {
	defer backend.endpoint.Close()
	for {
		// TODO: register connID and remoteID
		session, _, _, err := backend.endpoint.Accept()
		if err != nil {
			return
		}
		go handler.HandleSession(session)
	}
}

func (backend *Backend) GetSession(sessionID uint64) *link.Session {
	return backend.endpoint.GetSession(sessionID)
}

func (backend *Backend) Init(initializer func(FullService)) {
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
