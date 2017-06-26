package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/grpc"
	"github.com/evanphx/mesh/instance"
	"github.com/evanphx/mesh/pb"
)

var (
	fToken  = flag.String("token", "", "static token to use for auth")
	fNet    = flag.String("network", "", "network name to expect")
	fAddr   = flag.String("addr", ":0", "address to listen on for connections")
	fName   = flag.String("name", "", "name of the pipe use")
	fListen = flag.Bool("listen", false, "whether or not to listen on the pipe")
)

type ts struct{}

func (t *ts) StreamTimes(r *Req, str Timer_StreamTimesServer) error {
	log.Printf("starting time streaming....")
	defer log.Printf("end time streaming....")

	i := int64(0)
	for {
		err := str.Send(&Time{i})
		if err != nil {
			log.Printf("error sending to stream: %s", err)
			return err
		}

		time.Sleep(100 * time.Millisecond)
		i++
	}
}

func main() {
	flag.Parse()

	ctx := context.Background()

	inst, err := instance.InitNew()
	if err != nil {
		log.Fatal(err)
	}

	inst.ProvideInfo()

	inst.StaticTokenAuth(*fNet, *fToken)

	err = inst.ConnectTCP(ctx, *fAddr, *fNet)
	if err != nil {
		log.Fatal(err)
	}

	if *fListen {
		var t ts
		server := grpc.NewServer()
		RegisterTimerServer(server, &t)

		adver := &pb.Advertisement{}
		adver.Pipe = *fName

		err := inst.HandleRPC(ctx, adver, server)
		log.Fatal(err)

		return
	}

	time.Sleep(1000 * time.Millisecond)

	fmt.Fprintf(os.Stderr, "Connecting to %s\n", *fName)

	client := NewTimerClient(inst.RPCConnect(&mesh.PipeSelector{Pipe: *fName}))

	str, err := client.StreamTimes(ctx, &Req{})
	if err != nil {
		log.Fatal(err)
	}

	for {
		t, err := str.Recv()
		if err != nil {
			log.Fatalf("stream recv error: %s", err)
		}

		log.Printf("%v", t)
	}
}
