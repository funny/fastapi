package fastapi

import (
	"log"
	"net"
	"runtime/debug"

	fastway "github.com/funny/fastway/go"
	"github.com/funny/link"
	"github.com/funny/slab"
)

type App struct {
	serviceTypes []*ServiceType
	services     [256]Provider

	Pool         slab.Pool
	ReadBufSize  int
	SendChanSize int
	MaxRecvSize  int
	MaxSendSize  int
	Handler      Handler
}

func New() *App {
	return &App{
		Pool:         &slab.NoPool{},
		ReadBufSize:  1024,
		SendChanSize: 1024,
		MaxRecvSize:  64 * 1024,
		MaxSendSize:  64 * 1024,
		Handler:      &noHandler{},
	}
}

func (app *App) HandleRequest(session *link.Session, req Message) {
	app.services[req.ServiceID()].(Service).HandleRequest(session, req)
}

func (app *App) Dial(network, address string) (*link.Session, error) {
	return link.Dial(network, address, link.ProtocolFunc(app.newClientCodec), app.SendChanSize)
}

func (app *App) NewClient(conn net.Conn) (*link.Session, error) {
	codec, _ := app.newClientCodec(conn)
	return link.NewSession(codec, app.SendChanSize), nil
}

func (app *App) Listen(network, address string) (*link.Server, error) {
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return app.NewServer(listener)
}

func (app *App) NewServer(listener net.Listener) (*link.Server, error) {
	return link.NewServer(listener, link.ProtocolFunc(app.newServerCodec), app.SendChanSize), nil
}

func (app *App) NewFastwayServer(conn net.Conn, cfg fastway.EndPointCfg) (*fastway.EndPoint, error) {
	cfg.MsgFormat = &msgFormat{app.newRequest}
	return fastway.NewServer(conn, cfg)
}

func (app *App) NewFastwayClient(conn net.Conn, cfg fastway.EndPointCfg) (*fastway.EndPoint, error) {
	cfg.MsgFormat = &msgFormat{app.newRequest}
	return fastway.NewClient(conn, cfg), nil
}

type Handler interface {
	InitSession(*link.Session) error
	Transaction(*link.Session, func())
}

type noHandler struct {
}

func (t *noHandler) InitSession(session *link.Session) error {
	return nil
}

func (t *noHandler) Transaction(session *link.Session, work func()) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Unhandled fastapi error:", err)
			log.Println(string(debug.Stack()))
		}
	}()
	work()
}

func (app *App) HandleSessoin(session *link.Session, handler Handler) {
	defer session.Close()
	handler.InitSession(session)
	for {
		msg, err := session.Receive()
		if err != nil {
			return
		}
		handler.Transaction(session, func() {
			app.HandleRequest(session, msg.(Message))
		})
	}
}

func (app *App) Serve(server *link.Server, handler Handler) {
	if handler == nil {
		handler = &noHandler{}
	}
	server.Serve(link.HandlerFunc(func(session *link.Session) {
		app.HandleSessoin(session, handler)
	}))
}

func (app *App) ServeFastway(endpoint *fastway.EndPoint, handler Handler) {
	if handler == nil {
		handler = &noHandler{}
	}
	for {
		session, err := endpoint.Accept()
		if err != nil {
			return
		}
		go app.HandleSessoin(session.Session, handler)
	}
}
