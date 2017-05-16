package transport

import (
	"context"
	"net"

	"github.com/anacrolix/utp"
)

func ListenUTP(pctx context.Context, p Peer, v Validator, addr string) (net.Addr, error) {
	l, err := utp.NewSocket("udp", addr)
	if err != nil {
		return nil, err
	}

	return listen(pctx, p, v, l)
}

func ConnectUTP(ctx context.Context, l Peer, v Validator, host, netName string) error {
	conn, err := utp.Dial(host)
	if err != nil {
		return err
	}

	sess, err := connectHS(ctx, l, netName, v, NewFramer(conn))
	if err != nil {
		conn.Close()
		return err
	}

	l.AddSession(sess)

	return nil
}
