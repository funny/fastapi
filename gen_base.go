package fastapi

import (
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/funny/link"
	"github.com/funny/slab"
)

type APIs map[byte][2]interface{}

type Provider interface {
	APIs() APIs
}

type App struct {
	name         string
	sessionType  reflect.Type
	serviceTypes []*ServiceType
	services     [256]reflect.Type
	Config
}

func New() *App {
	return &App{
		sessionType: reflect.TypeOf(&link.Session{}),
		Config: Config{
			Pool:         &slab.NoPool{},
			ReadBufSize:  1024,
			SendChanSize: 1024,
			MaxRecvSize:  64 * 1024,
			MaxSendSize:  64 * 1024,
		},
	}
}

func (app *App) Register(id byte, service Provider) {
	typeOfService := reflect.TypeOf(service)

	if app.services[id] != nil {
		panic(fmt.Sprintf("duplicate service id '%d' for '%s' and '%s'", id, typeOfService, app.services[id]))
	}

	for _, serviceType := range app.serviceTypes {
		if serviceType.t == typeOfService {
			panic(fmt.Sprintf("duplicate register in '%s'", typeOfService))
		}
	}

	app.services[id] = typeOfService.Elem()

	serviceType := &ServiceType{
		id:          id,
		t:           typeOfService,
		sessionType: app.sessionType,
	}

	for id, api := range service.APIs() {
		if api[0] != nil {
			serviceType.registerReq(id, api[0])
		}
		if api[1] != nil {
			serviceType.registerRsp(id, api[1])
		}
	}

	app.serviceTypes = append(app.serviceTypes, serviceType)
}

func (app *App) Services() []*ServiceType {
	return app.serviceTypes
}

type ServiceType struct {
	id          byte
	t           reflect.Type
	sessionType reflect.Type
	requests    []*MessageType
	responses   []*MessageType
	handlers    []*HandlerMethod
}

func (service *ServiceType) registerReq(id byte, req interface{}) {
	reqType := reflect.TypeOf(req)
	if reqType.Kind() == reflect.Ptr {
		reqType = reqType.Elem()
	}

	for _, req := range service.requests {
		if req.t == reqType {
			panic(fmt.Sprintf("duplicate register request type '%s'", reqType))
		}
	}

	service.requests = append(service.requests, &MessageType{
		service: service,
		id:      id,
		t:       reqType,
	})

	// Search Request Handler:
	//
	//     HandleRequest(session *MySession, req *MyRequest) *MyResponse
	//     HandleRequest(req *MyRequest) *MyResponse
	//     HandleRequest(session *MySession, req *MyRequest)
	//     HandleRequest(req *MyRequest)
	//
	for i := 0; i < service.t.NumMethod(); i++ {
		method := service.t.Method(i)

		if n := method.Type.NumOut(); n != 0 && n != 1 {
			continue
		}

		if n := method.Type.NumIn() - 1; n != 1 && n != 2 {
			continue
		}

		if lastArg := method.Type.In(method.Type.NumIn() - 1); lastArg.Kind() != reflect.Ptr || lastArg.Elem() != reqType {
			continue
		}

		var rspType reflect.Type
		if method.Type.NumOut() == 1 {
			rspType = method.Type.Out(0)
		}

		service.handlers = append(service.handlers, &HandlerMethod{
			ID:          id,
			Name:        method.Name,
			ReqType:     reqType,
			RspType:     rspType,
			NeedSession: method.Type.In(1) == service.sessionType,
		})
		break
	}
}

func (service *ServiceType) registerRsp(id byte, rsp interface{}) {
	rspType := reflect.TypeOf(rsp)
	if rspType.Kind() == reflect.Ptr {
		rspType = rspType.Elem()
	}

	for _, rsp := range service.responses {
		if rsp.t == rspType {
			panic(fmt.Sprintf("duplicate register response type '%s'", rspType))
		}
	}

	service.responses = append(service.responses, &MessageType{
		service: service,
		id:      id,
		t:       rspType,
	})
}

func (service *ServiceType) ID() byte {
	return service.id
}

func (service *ServiceType) Type() reflect.Type {
	return service.t.Elem()
}

func (service *ServiceType) SessionType() reflect.Type {
	return service.sessionType
}

func (service *ServiceType) Package() string {
	return filepath.Base(service.t.PkgPath())
}

func (service *ServiceType) Name() string {
	return service.t.Elem().Name()
}

func (service *ServiceType) Requests() []*MessageType {
	return service.requests
}

func (service *ServiceType) Responses() []*MessageType {
	return service.responses
}

func (service *ServiceType) Handlers() []*HandlerMethod {
	return service.handlers
}

type MessageType struct {
	service *ServiceType
	id      byte
	t       reflect.Type
}

func (msg *MessageType) Service() *ServiceType {
	return msg.service
}

func (msg *MessageType) ID() byte {
	return msg.id
}

func (msg *MessageType) Type() reflect.Type {
	return msg.t
}

func (msg *MessageType) Package() string {
	return filepath.Base(msg.t.PkgPath())
}

func (msg *MessageType) Name() string {
	return msg.t.Name()
}

type HandlerMethod struct {
	ID          byte
	Name        string
	ReqType     reflect.Type
	RspType     reflect.Type
	NeedSession bool
}

func (h *HandlerMethod) InvokeCode() string {
	if !h.NeedSession {
		if h.RspType == nil {
			return fmt.Sprintf("s.%s(req.(*%s))", h.Name, h.ReqType.Name())
		}
		return fmt.Sprintf("session.Send(s.%s(req.(*%s)))", h.Name, h.ReqType.Name())
	}

	if h.RspType == nil {
		return fmt.Sprintf("s.%s(session, req.(*%s))", h.Name, h.ReqType.Name())
	}
	return fmt.Sprintf("session.Send(s.%s(session, req.(*%s)))", h.Name, h.ReqType.Name())
}
