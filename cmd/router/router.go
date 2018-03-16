package main

import (
	"flag"
	"log"

	"github.com/evanphx/mesh/instance"
)

var (
	fToken = flag.String("token", "", "static token to use for auth")
	fNet   = flag.String("network", "", "network name to expect")
	fAddr  = flag.String("addr", ":0", "address to listen on for connections")
)

func main() {
	flag.Parse()

	inst, err := instance.InitNew()
	if err != nil {
		log.Fatal(err)
	}

	// inst.ProvideInfo()

	// inst.StaticTokenAuth(*fNet, *fToken)

	_, err = inst.ListenTCP(*fAddr, []string{*fNet})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Listening\n")

	select {}
}
