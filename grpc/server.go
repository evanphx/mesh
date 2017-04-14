package grpc

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"golang.org/x/net/context"
)

// UnaryServerInfo consists of various information about a unary RPC on
// server side. All per-rpc information may be mutated by the interceptor.
type UnaryServerInfo struct {
	// Server is the service implementation the user provides. This is read-only.
	Server interface{}
	// FullMethod is the full RPC method string, i.e., /package.service/method.
	FullMethod string
}

// UnaryHandler defines the handler invoked by UnaryServerInterceptor to complete the normal
// execution of a unary RPC.
type UnaryHandler func(ctx context.Context, req interface{}) (interface{}, error)

// UnaryServerInterceptor provides a hook to intercept the execution of a unary RPC on the server. info
// contains all the information of this RPC the interceptor can operate on. And handler is the wrapper
// of the service method implementation. It is the responsibility of the interceptor to invoke handler
// to complete the RPC.
type UnaryServerInterceptor func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (resp interface{}, err error)

func unaryInt(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
	return handler(ctx, req)
}

type HandlerFunc func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor UnaryServerInterceptor) (interface{}, error)

type MethodDesc struct {
	MethodName string
	Handler    HandlerFunc
}

type ServiceDesc struct {
	ServiceName string
	HandlerType interface{}
	Methods     []MethodDesc
	Streams     []StreamDesc
	Metadata    string
}

type ServiceRegistry interface {
	RegisterService(desc ServiceDesc, srv interface{})
}

// service consists of the information of the server serving this service and
// the methods in this service.
type service struct {
	server interface{} // the server for service methods
	md     map[string]*MethodDesc
	sd     map[string]*StreamDesc
	mdata  interface{}
}

type Server struct {
	lock sync.Mutex

	services map[string]*service
}

func (s *Server) RegisterService(desc ServiceDesc, srv interface{}) {
	s.lock.Lock()
	defer s.lock.Unlock()

	srv := &service{
		server: ss,
		md:     make(map[string]*MethodDesc),
		sd:     make(map[string]*StreamDesc),
		mdata:  sd.Metadata,
	}

	for i := range sd.Methods {
		d := &sd.Methods[i]
		srv.md[d.MethodName] = d
	}

	for i := range sd.Streams {
		d := &sd.Streams[i]
		srv.sd[d.StreamName] = d
	}

	s.services[sd.ServiceName] = srv
}

func (s *Server) handleStream(ctx context.Context, stream Stream) {
	sm := stream.Method()
	if sm != "" && sm[0] == '/' {
		sm = sm[1:]
	}
	pos := strings.LastIndex(sm, "/")
	if pos == -1 {
		stream.SendError(ctx, fmt.Errorf("malformed method name: %q", stream.Method()))
		return
	}

	service := sm[:pos]
	method := sm[pos+1:]
	srv, ok := s.m[service]
	if !ok {
		stream.SendError(ctx, fmt.Errorf("unknown service %v", service))
		return
	}

	// Unary RPC or Streaming RPC?
	if md, ok := srv.md[method]; ok {
		s.processUnaryRPC(ctx, stream, srv, md)
		return
	}

	/*
		if sd, ok := srv.sd[method]; ok {
			s.processStreamingRPC(t, stream, srv, sd, trInfo)
			return
		}
	*/

	stream.SendError(ctx, fmt.Errorf("unknown method %v", method))
}

type Unmarshaler interface {
	Unmarshal(req []byte) error
}

func pbUnmarshal(v interface{}) error {
	if m, ok := v.(Unmarshaler); ok {
		return m.Unmarshal(req)
	}

	return fmt.Errorf("Invalid request type: %T", v)
}

func (s *Server) processUnaryRPC(ctx context.Context, stream Stream, srv *service, md *MethodDesc) (err error) {
	req, err := stream.RecvMsg(ctx, s.opts.maxMsgSize)
	if err != nil {
		if err == io.EOF {
			// The entire stream is done (for unary RPC only).
			return err
		}

		err = stream.SendError(ctx, err)
		if err != nil {
			log.Printf("mesh-grpc: Error sending err back to client: %s", err)
		}

		return err
	}

	reply, appErr := md.Handler(srv.server, ctx, pbUnmarshal, nil)
	if appErr != nil {
		err = stream.SendError(ctx, appErr)
		if appErr != nil {
			log.Printf("mesh-grpc: Error sending err back to client: %s", err)
		}

		return appErr
	}

	return stream.SendReply(ctx, reply)
}
