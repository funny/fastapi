package fastapi

import (
	"log"
	"net"
	"runtime/debug"

	fastway "github.com/funny/fastway/go"
	"github.com/funny/link"
	"github.com/funny/slab"
)

type Transaction interface {
	Transaction(*link.Session, func())
}

type noTrans struct {
}

func (t *noTrans) Transaction(session *link.Session, work func()) {
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
	Transaction  Transaction
}

func New() *App {
	return &App{
		Pool:         &slab.NoPool{},
		ReadBufSize:  1024,
		SendChanSize: 1024,
		MaxRecvSize:  64 * 1024,
		MaxSendSize:  64 * 1024,
		Transaction:  &noTrans{},
	}
}

func (app *App) Dial(network, address string) (*link.Session, error) {
	return link.Dial(network, address, link.ProtocolFunc(app.newClientCodec), app.SendChanSize)
}

func (app *App) NewClient(conn net.Conn) *link.Session {
	codec, _ := app.newClientCodec(conn)
	return link.NewSession(codec, app.SendChanSize)
}

func (app *App) ListenAndServe(network, address string) (*link.Server, error) {
	lsn, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return app.ServeListener(lsn), nil
}

func (app *App) ServeListener(listener net.Listener) *link.Server {
	server := link.NewServer(listener, link.ProtocolFunc(app.newServerCodec), app.SendChanSize)
	go server.Serve(app)
	return server
}

func (app *App) ServeFastway(conn net.Conn, cfg fastway.EndPointCfg) (*fastway.EndPoint, error) {
	cfg.MsgFormat = &msgFormat{app.newRequest}

	endpoint, err := fastway.NewServer(conn, cfg)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			conn, err := endpoint.Accept()
			if err != nil {
				return
			}
			go app.HandleSession(conn.Session)
		}
	}()

	return endpoint, nil
}

func (app *App) HandleSession(session *link.Session) {
	defer session.Close()
	for {
		msg, err := session.Receive()
		if err != nil {
			break
		}
		app.Transaction.Transaction(session, func() {
			req := msg.(Message)
			app.services[req.ServiceID()].(Service).HandleRequest(session, req)
		})
	}
}
