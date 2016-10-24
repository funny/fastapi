package main

import (
	"flag"
	"log"

	"github.com/funny/fastapi"
	"github.com/funny/fastapi/example/fastapi_toy/module1"
	"github.com/funny/fastbin"
	"github.com/funny/link"
)

func main() {
	gencode := flag.Bool("gencode", false, "generate code")
	flag.Parse()

	app := fastapi.New()
	app.Register(1, &module1.Service{})

	if *gencode {
		fastapi.GenCode(app)
		fastbin.GenCode()
		return
	}

	server, err := app.Listen("tcp", "0.0.0.0:0")
	checkErr(err)
	go server.Serve(link.HandlerFunc(func(session *link.Session) {
		serverSessionLoop(server, session)
	}))

	client := app.NewClient()
	addr := server.Listener().Addr().String()
	session, err := client.Dial("tcp", addr)
	checkErr(err)
	clientSessionLoop(session)
}

func serverSessionLoop(server *fastapi.Server, session *link.Session) {
	for {
		msg, err := session.Receive()
		checkErr(err)
		server.Dispatch(session, msg.(fastapi.Message))
	}
}

func clientSessionLoop(session *link.Session) {
	for i := 0; i < 10; i++ {
		err := session.Send(&module1.AddReq{i, i})
		checkErr(err)

		rsp, err := session.Receive()
		checkErr(err)

		log.Printf("AddRsp: %d", rsp.(*module1.AddRsp).C)
	}
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
