package fastapi

import (
	"errors"
	"log"
	"net"
	"runtime/debug"
	"sync"

	fastway "github.com/funny/fastway/go"
	"github.com/funny/link"
	"github.com/funny/slab"
)

var ErrAppStopped = errors.New("app stopped")

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

type App struct {
	serviceTypes []*ServiceType
	services     [256]Provider

	Pool         slab.Pool
	ReadBufSize  int
	SendChanSize int
	MaxRecvSize  int
	MaxSendSize  int
	Handler      Handler

	stopMutex sync.RWMutex
	stopped   bool
	clients   *link.Channel
	servers   []*link.Server
	endpoints []*fastway.EndPoint
}

func New() *App {
	return &App{
		Pool:         &slab.NoPool{},
		ReadBufSize:  1024,
		SendChanSize: 1024,
		MaxRecvSize:  64 * 1024,
		MaxSendSize:  64 * 1024,
		Handler:      &noHandler{},
		clients:      link.NewChannel(),
	}
}

func (app *App) Dial(network, address string) (*link.Session, error) {
	app.stopMutex.RLock()
	defer app.stopMutex.RUnlock()
	if app.stopped {
		return nil, ErrAppStopped
	}

	client, err := link.Dial(network, address, link.ProtocolFunc(app.newClientCodec), app.SendChanSize)
	if err != nil {
		return nil, err
	}

	app.clients.Put(client.ID(), client)
	return client, nil
}

func (app *App) NewClient(conn net.Conn) (*link.Session, error) {
	app.stopMutex.RLock()
	defer app.stopMutex.RUnlock()
	if app.stopped {
		return nil, ErrAppStopped
	}

	codec, _ := app.newClientCodec(conn)
	client := link.NewSession(codec, app.SendChanSize)

	app.clients.Put(client.ID(), client)
	return client, nil
}

func (app *App) ListenAndServe(network, address string) error {
	app.stopMutex.RLock()
	defer app.stopMutex.RUnlock()
	if app.stopped {
		return ErrAppStopped
	}

	listener, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	app.serveListener(listener)
	return nil
}

func (app *App) ServeListener(listener net.Listener) error {
	app.stopMutex.RLock()
	defer app.stopMutex.RUnlock()
	if app.stopped {
		return ErrAppStopped
	}
	app.serveListener(listener)
	return nil
}

func (app *App) serveListener(listener net.Listener) {
	server := link.NewServer(listener, link.ProtocolFunc(app.newServerCodec), app.SendChanSize)
	app.servers = append(app.servers, server)
	go func() {
		defer func() {
			for i, s := range app.servers {
				if s == server {
					copy(app.servers[i:], app.servers[i+1:])
					app.servers = app.servers[:len(app.servers)-1]
				}
			}
		}()
		server.Serve(app)
	}()
}

func (app *App) ServeFastway(conn net.Conn, cfg fastway.EndPointCfg) error {
	app.stopMutex.RLock()
	defer app.stopMutex.RUnlock()
	if app.stopped {
		return ErrAppStopped
	}

	cfg.MsgFormat = &msgFormat{app.newRequest}

	endpoint, err := fastway.NewServer(conn, cfg)
	if err != nil {
		return err
	}

	app.endpoints = append(app.endpoints, endpoint)
	go func() {
		defer func() {
			for i, p := range app.endpoints {
				if p == endpoint {
					copy(app.endpoints[i:], app.endpoints[i+1:])
					app.endpoints = app.endpoints[:len(app.endpoints)-1]
				}
			}
		}()
		for {
			conn, err := endpoint.Accept()
			if err != nil {
				return
			}
			go app.HandleSession(conn.Session)
		}
	}()
	return nil
}

func (app *App) HandleSession(session *link.Session) {
	defer session.Close()

	if err := app.Handler.InitSession(session); err != nil {
		return
	}

	for {
		msg, err := session.Receive()
		if err != nil {
			break
		}
		app.Handler.Transaction(session, func() {
			req := msg.(Message)
			app.services[req.ServiceID()].(Service).HandleRequest(session, req)
		})
	}
}

func (app *App) Stop() {
	app.stopMutex.Lock()
	defer app.stopMutex.Unlock()

	if app.stopped {
		return
	}
	app.stopped = true

	app.clients.FetchAndRemove(func(client *link.Session) {
		client.Close()
	})

	for _, server := range app.servers {
		server.Stop()
	}

	for _, endpoint := range app.endpoints {
		endpoint.Close()
	}
}

func (app *App) LastServerAddr() net.Addr {
	app.stopMutex.RLock()
	defer app.stopMutex.RUnlock()
	return app.servers[len(app.servers)-1].Listener().Addr()
}
