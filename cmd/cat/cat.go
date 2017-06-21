package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/evanphx/mesh"
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

func main() {
	flag.Parse()

	inst, err := instance.InitNew()
	if err != nil {
		log.Fatal(err)
	}

	inst.ProvideInfo()

	inst.StaticTokenAuth(*fNet, *fToken)

	ctx := context.Background()

	err = inst.ConnectTCP(ctx, *fAddr, *fNet)
	if err != nil {
		log.Fatal(err)
	}

	if *fListen {
		adver := &pb.Advertisement{}
		adver.Pipe = *fName
		lp, err := inst.Listen(adver)
		if err != nil {
			log.Fatal(err)
		}

		for {
			p, err := lp.Accept(ctx)
			if err != nil {
				log.Fatal(err)
			}

			for {
				msg, err := p.Recv(ctx)
				if err != nil {
					break
				}

				os.Stdout.Write(msg)
			}
		}

		return
	}

	time.Sleep(1000 * time.Millisecond)

	fmt.Fprintf(os.Stderr, "Connecting to %s\n", *fName)

	pipe, err := inst.Connect(ctx, &mesh.PipeSelector{Pipe: *fName})
	if err != nil {
		log.Fatal(err)
	}

	br := bufio.NewReader(os.Stdin)

	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			break
		}

		pipe.Send(ctx, line)
	}
}
