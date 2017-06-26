package grpc

import (
	"context"

	"github.com/evanphx/mesh/log"
)

// CallOption configures a Call before it starts or extracts information from
// a Call after it completes.
type CallOption interface{}

type Transport interface {
	Recv(context.Context) ([]byte, error)
	Send(context.Context, []byte) error
	SendFinal(context.Context, []byte) error
	Close(context.Context) error
}

type ClientConn struct {
	tr      Transport
	creator func(context.Context) (Transport, error)
}

func NewClientConn(tr Transport) *ClientConn {
	return &ClientConn{tr: tr}
}

func NewOnDemandClientConn(f func(context.Context) (Transport, error)) *ClientConn {
	return &ClientConn{creator: f}
}

type ClientStream interface {
	SendMsg(m interface{}) error
	RecvMsg(reply interface{}) error
	CloseSend() error
}

func NewClientStream(ctx context.Context, desc *StreamDesc, cc *ClientConn, method string, opts ...CallOption) (ClientStream, error) {
	cs, err := cc.makeStream(ctx, method)
	if err != nil {
		return nil, err
	}

	err = cs.SendRequest(ctx)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (cc *ClientConn) makeStream(ctx context.Context, method string) (*clientStream, error) {
	tr := cc.tr

	var err error

	if tr == nil {
		tr, err = cc.creator(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &clientStream{ctx: ctx, tr: tr, method: method}, nil
}

// Invoke sends the RPC request on the wire and returns after response is received.
// Invoke is called by generated code. Also users can call Invoke directly when it
// is really needed in their use cases.
func Invoke(ctx context.Context, method string, args, reply interface{}, cc *ClientConn, opts ...CallOption) error {
	return invoke(ctx, method, args, reply, cc, opts...)
}

func invoke(ctx context.Context, method string, args, reply interface{}, cc *ClientConn, opts ...CallOption) (e error) {
	stream, err := cc.makeStream(ctx, method)
	if err != nil {
		log.Debugf("Unable to create stream for invoke: %s", err)
		return err
	}

	err = stream.SendRequest(ctx)
	if err != nil {
		return err
	}

	log.Debugf("invoke sending args...")

	err = stream.SendMsg(args)
	if err != nil {
		return err
	}

	err = stream.RecvMsg(reply)
	if err != nil {
		return err
	}

	return stream.Close(ctx)
}
