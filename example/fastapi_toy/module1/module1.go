package module1

import (
	"github.com/funny/fastapi"
	"github.com/funny/link"
)

type Service struct {
}

func (_ *Service) APIs() fastapi.APIs {
	return fastapi.APIs{
		1: {AddReq{}, AddRsp{}},
	}
}

type AddReq struct {
	A int
	B int
}

type AddRsp struct {
	C int
}

func (_ *Service) Add(session *link.Session, req *AddReq) *AddRsp {
	return &AddRsp{
		req.A + req.B,
	}
}
