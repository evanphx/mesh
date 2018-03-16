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

	ctx := context.Background()

	if *fListen {
		err = inst.FindNodes(ctx, *fNet)
		if err != nil {
			log.Fatal(err)
		}
		/*
			err := inst.Bind()
			if err != nil {
				log.Fatal(err)
			}
		*/

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

			go func() {
				br := bufio.NewReader(os.Stdin)

				for {
					line, err := br.ReadBytes('\n')
					if err != nil {
						break
					}

					p.Send(ctx, line)
				}
			}()

			for {
				msg, err := p.Recv(ctx)
				if err != nil {
					os.Exit(1)
				}

				os.Stdout.Write(msg)
			}
		}
	}

	time.Sleep(1000 * time.Millisecond)

	err = inst.FindNodes(ctx, *fNet)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(os.Stderr, "Connecting to %s\n", *fName)

	time.Sleep(1 * time.Second)

	pipe, err := inst.Connect(ctx, &mesh.PipeSelector{Pipe: *fName})
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			msg, err := pipe.Recv(ctx)
			if err != nil {
				log.Printf("error on recv: %s\n", err)
				return
			}

			os.Stdout.Write(msg)
		}
	}()

	br := bufio.NewReader(os.Stdin)

	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			break
		}

		err = pipe.Send(ctx, line)
		if err != nil {
			log.Printf("error on send: %s\n", err)
		}
	}
}
