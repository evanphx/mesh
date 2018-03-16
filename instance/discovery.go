package instance

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/evanphx/mesh/discovery/mdns"
)

func (i *Instance) FindNodes(ctx context.Context, network string) error {
	entries := make(chan *mdns.ServiceEntry, 20)
	go func() {
		err := mdns.Lookup("_mesh._tcp", entries)
		if err != nil {
			panic(err)
		}

		close(entries)
	}()

	// defer fmt.Printf("finished with find nodes\n")

	// log.Printf("looking for nodes with network=%s\n", network)

	match := fmt.Sprintf("network=%s", network)

	for {
		if ent, ok := <-entries; ok {
			for _, info := range ent.InfoFields {
				// log.Printf("info on %s:%d: %s\n", ent.Addr, ent.Port, info)
				if info == match {
					host := net.JoinHostPort(ent.Addr.String(), strconv.Itoa(ent.Port))
					// log.Printf("connecting...\n")
					return i.ConnectTCP(ctx, host, network)
				}
			}
		} else {
			panic("no nodes found")
		}
	}

	return nil
}
