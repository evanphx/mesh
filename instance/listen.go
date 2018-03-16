package instance

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/evanphx/mesh/discovery/mdns"
	"github.com/evanphx/mesh/transport"
	"github.com/miekg/dns"
)

type zoneList []mdns.Zone

func (z *zoneList) Records(q dns.Question) []dns.RR {
	var out []dns.RR

	for _, zone := range *z {
		out = append(out, zone.Records(q)...)
	}

	return out
}

var localInstances *zoneList

var mdnsServer *mdns.Server

func init() {
	localInstances = new(zoneList)
	serv, err := mdns.NewServer(&mdns.Config{
		Zone: localInstances,
	})

	if err != nil {
		panic(err)
	}

	mdnsServer = serv
}

func (i *Instance) implicitNetworks() []string {
	cfg := LoadConfig()

	str := os.Getenv("MESH_NETWORKS")

	networks := strings.Split(str, ",")

	return append(networks, cfg.Networks...)
}

func (i *Instance) checkListeners() error {
	i.lock.Lock()
	defer i.lock.Unlock()

	if i.hasListeners {
		return nil
	}

	_, err := i.ListenTCP(":0", i.implicitNetworks())
	if err == nil {
		i.hasListeners = true
	}

	return err
}

func (i *Instance) Bind() error {
	return i.checkListeners()
}

func (i *Instance) ListenTCP(addr string, networks []string) (*net.TCPAddr, error) {
	a, err := transport.ListenTCP(i.lifetime, i.Peer, i.validator, addr)
	if err != nil {
		return nil, err
	}

	tcpaddr := a.(*net.TCPAddr)

	if i.options.AdvertiseMDNS {
		var txts []string

		for _, n := range networks {
			txts = append(txts, fmt.Sprintf("network=%s", n))
		}

		ip := net.ParseIP("127.0.0.1")

		serv, err := mdns.NewMDNSService("mesh instance", "_mesh._tcp", "local.", "", tcpaddr.Port, []net.IP{ip}, txts)
		if err != nil {
			return nil, err
		}

		*localInstances = append(*localInstances, serv)
	}

	return tcpaddr, nil
}
