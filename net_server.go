package fastapi

import (
	"fmt"
	"net"
	"reflect"

	"github.com/funny/link"
)

var _ IServer = (*Server)(nil)

type Server struct {
	*link.Server
	services [256]Service
}

func (app *App) Listen(network, address string) (*Server, error) {
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return app.NewServer(listener), nil
}

func (app *App) NewServer(listener net.Listener) *Server {
	server := new(Server)
	server.Server = link.NewServer(listener, &protocol{&app.Config, server.newMsg}, app.Config.SendChanSize)
	for i, s := range app.services {
		if s != nil {
			server.services[i] = reflect.New(s).Interface().(Service)
		}
	}
	return server
}

func (server *Server) Init(initializer func(Service)) {
	for _, s := range server.services {
		if s != nil {
			initializer(s)
		}
	}
}

func (server *Server) Dispatch(session *link.Session, req Message) {
	server.services[req.ServiceID()].HandleRequest(session, req)
}

func (server *Server) newMsg(serviceID, messageID byte) (Message, error) {
	if service := server.services[serviceID]; service != nil {
		if msg := service.NewRequest(messageID); msg != nil {
			return msg, nil
		}
		return nil, DecodeError{fmt.Sprintf("Unsupported Message Type: [%d, %d]", serviceID, messageID)}
	}
	return nil, DecodeError{fmt.Sprintf("Unsupported Service: [%d, %d]", serviceID, messageID)}
}
