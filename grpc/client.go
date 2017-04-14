package grpc

import "context"

// CallOption configures a Call before it starts or extracts information from
// a Call after it completes.
type CallOption interface {
	// before is called before the call is sent to any server.  If before
	// returns a non-nil error, the RPC fails with that error.
	before(*callInfo) error

	// after is called after the call has completed.  after cannot return an
	// error, so any failures should be reported via output parameters.
	after(*callInfo)
}

type ClientConn struct{}

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
