package main

import (
	"flag"
	"log"
	"os"

	"github.com/funny/fastapi"
	"github.com/funny/fastapi/example/fastapi_toy/module1"
	"github.com/funny/fastbin"
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
	if err != nil {
		log.Fatal("setup server failed:", err)
	}
	go app.Serve(server, nil)

	client, err := app.Dial("tcp", server.Listener().Addr().String())
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

	server.Stop()

	log.Printf("============")
	app.TimeRecoder().WriteCSV(os.Stderr)
}
