package fastapi

import (
	"fmt"
	"net"
	"reflect"

	"github.com/funny/link"
)

type Client struct {
	*Config
	services [256]Service
	protocol *protocol
}

func (app *App) NewClient() *Client {
	client := &Client{
		Config: &app.Config,
	}
	for i, s := range app.services {
		if s != nil {
			client.services[i] = reflect.New(s).Interface().(Service)
		}
	}
	client.protocol = &protocol{&app.Config, client.newMsg}
	return client
}

func (client *Client) Dial(network, address string) (*link.Session, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return client.NewSession(conn), nil
}

func (client *Client) NewSession(conn net.Conn) *link.Session {
	codec, _ := client.protocol.NewCodec(conn)
	return link.NewSession(codec, client.SendChanSize)
}

func (client *Client) Init(initializer func(Service)) {
	for _, s := range client.services {
		if s != nil {
			initializer(s)
		}
	}
}

func (client *Client) newMsg(serviceID, messageID byte) (Message, error) {
	if service := client.services[serviceID]; service != nil {
		if msg := service.NewResponse(messageID); msg != nil {
			return msg, nil
		}
		return nil, DecodeError{fmt.Sprintf("Unsupported Message Type: [%d, %d]", serviceID, messageID)}
	}
	return nil, DecodeError{fmt.Sprintf("Unsupported Service: [%d, %d]", serviceID, messageID)}
}
