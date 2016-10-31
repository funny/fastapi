package main

import (
	"flag"
	"log"

	"github.com/funny/fastapi"
	"github.com/funny/fastbin"
)

import (
	"github.com/funny/fastapi/example/fastapi_toy/module1"
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

	err := app.ListenAndServe("tcp", "0.0.0.0:0")
	if err != nil {
		log.Fatal("setup server failed:", err)
	}

	client, err := app.Dial("tcp", app.LastServerAddr().String())
	if err != nil {
		log.Fatal("setup client failed:", err)
	}

	for i := 0; i < 10; i++ {
		err := client.Send(&module1.AddReq{i, i})
		if err != nil {
			log.Fatal("send failed:", err)
		}

		rsp, err := client.Receive()
		if err != nil {
			log.Fatal("recv failed:", err)
		}

		log.Printf("AddRsp: %d", rsp.(*module1.AddRsp).C)
	}

	app.Stop()
}
