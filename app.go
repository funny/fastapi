package fastapi

import (
	"log"
	"net"
	"runtime/debug"
	"time"

	fastway "github.com/funny/fastway/go"
	"github.com/funny/link"
	"github.com/funny/pprof"
	"github.com/funny/slab"
)

type Handler interface {
	InitSession(*link.Session) error
	DropSession(*link.Session)
	Transaction(*link.Session, func())
}

type App struct {
	serviceTypes []*ServiceType
	services     [256]Provider
	timeRecoder  *pprof.TimeRecorder

	Pool         slab.Pool
	ReadBufSize  int
	SendChanSize int
	MaxRecvSize  int
	MaxSendSize  int
	RecvTimeout  time.Duration
	SendTimeout  time.Duration
}

func New() *App {
	return &App{
		timeRecoder:  pprof.NewTimeRecorder(),
		Pool:         &slab.NoPool{},
		ReadBufSize:  1024,
		SendChanSize: 1024,
		MaxRecvSize:  64 * 1024,
		MaxSendSize:  64 * 1024,
	}
}

func (app *App) handleSessoin(session *link.Session, handler Handler) {
	defer session.Close()

	if handler.InitSession(session) != nil {
		return
	}

	defer handler.DropSession(session)

	for {
		msg, err := session.Receive()
		if err != nil {
			return
		}

		handler.Transaction(session, func() {
			startTime := time.Now()
			req := msg.(Message)
			app.services[req.ServiceID()].(Service).HandleRequest(session, req)
			app.timeRecoder.Record(req.Identity(), time.Since(startTime))
		})
	}
}

func (app *App) TimeRecoder() *pprof.TimeRecorder {
	return app.timeRecoder
}

func (app *App) Dial(network, address string) (*link.Session, error) {
	return link.Dial(network, address, link.ProtocolFunc(app.newClientCodec), app.SendChanSize)
}

func (app *App) Listen(network, address string, handler Handler) (*link.Server, error) {
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return app.NewServer(listener, handler), nil
}

func (app *App) NewClient(conn net.Conn) *link.Session {
	codec, _ := app.newClientCodec(conn)
	return link.NewSession(codec, app.SendChanSize)
}

func (app *App) NewServer(listener net.Listener, handler Handler) *link.Server {
	if handler == nil {
		handler = &noHandler{}
	}
	return link.NewServer(
		listener, link.ProtocolFunc(app.newServerCodec), app.SendChanSize,
		link.HandlerFunc(func(session *link.Session) {
			app.handleSessoin(session, handler)
		}),
	)
}

func (app *App) NewFastwayClient(conn net.Conn, cfg fastway.EndPointCfg) *fastway.EndPoint {
	cfg.MsgFormat = &msgFormat{app.newRequest}
	return fastway.NewClient(conn, cfg)
}

func (app *App) NewFastwayServer(conn net.Conn, cfg fastway.EndPointCfg, handler Handler) (*FastwayServer, error) {
	cfg.MsgFormat = &msgFormat{app.newRequest}
	endpoint, err := fastway.NewServer(conn, cfg)
	if err != nil {
		return nil, err
	}
	if handler == nil {
		handler = &noHandler{}
	}
	return &FastwayServer{app, endpoint, handler}, nil
}

type FastwayServer struct {
	app      *App
	endpoint *fastway.EndPoint
	handler  Handler
}

func (s *FastwayServer) Serve() error {
	for {
		session, err := s.endpoint.Accept()
		if err != nil {
			return err
		}
		go s.app.handleSessoin(session, s.handler)
	}
}

func (s *FastwayServer) GetSession(sessionID uint64) *link.Session {
	return s.endpoint.GetSession(sessionID)
}

func (s *FastwayServer) Stop() {
	s.endpoint.Close()
}

type noHandler struct {
}

func (t *noHandler) DropSession(session *link.Session) {
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
