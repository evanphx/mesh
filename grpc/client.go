package grpc

import "context"

// CallOption configures a Call before it starts or extracts information from
// a Call after it completes.
type CallOption interface{}

type Transport interface {
	Recv(context.Context) ([]byte, error)
	Send([]byte) error
	SendFinal([]byte) error
	Close() error
}

type ClientConn struct {
	tr Transport
}

func NewClientConn(tr Transport) *ClientConn {
	return &ClientConn{tr}
}

type ClientStream interface {
	SendRequest(ctx context.Context, args interface{}) error
	ReadReply(ctx context.Context, reply interface{}) error
	Close() error
}

func (cc *ClientConn) makeStream(ctx context.Context, method string) (ClientStream, error) {
	return &clientStream{cc.tr, method}, nil
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
		return err
	}

	err = stream.SendRequest(ctx, args)
	if err != nil {
		return err
	}

	err = stream.ReadReply(ctx, reply)
	if err != nil {
		return err
	}

	return stream.Close()
}
