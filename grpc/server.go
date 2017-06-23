package grpc

import (
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"strings"
	"sync"

	"github.com/evanphx/mesh/log"
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

type ServerStream interface {
	RecvMsg(reply interface{}) error
	SendMsg(reply interface{}) error
}

// StreamHandler defines the handler called by gRPC server to complete the
// execution of a streaming RPC.
type StreamHandler func(srv interface{}, stream ServerStream) error

// StreamDesc represents a streaming RPC service's method specification.
type StreamDesc struct {
	StreamName string
	Handler    StreamHandler

	// At least one of these is true.
	ServerStreams bool
	ClientStreams bool
}

type ServiceDesc struct {
	ServiceName string
	HandlerType interface{}
	Methods     []MethodDesc
	Streams     []StreamDesc
	Metadata    string
}

type ServiceRegistry interface {
	RegisterService(desc *ServiceDesc, srv interface{})
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

func NewServer() *Server {
	return &Server{
		services: make(map[string]*service),
	}
}

func (s *Server) RegisterService(sd *ServiceDesc, ss interface{}) {
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

func (s *Server) HandleTransport(ctx context.Context, tr Transport) {
	msg, err := tr.Recv(ctx)
	if err != nil {
		log.Printf("Error receiving transport data: %s", err)
		return
	}

	var frame Frame

	err = frame.Unmarshal(msg)
	if err != nil {
		log.Printf("Error decoding frame: %s", err)
		return
	}

	if frame.Type != REQUEST {
		log.Printf("Invalid opening frame type: %s", frame.Type)
		tr.Close(ctx)
	}

	ss := &serverStream{
		frame: &frame,
		tr:    tr,
	}

	s.handleStream(ctx, ss)
}

func (s *Server) handleStream(ctx context.Context, stream Stream) {
	sm := stream.Method()
	log.Debugf("handling request for: %s", sm)

	if sm != "" && sm[0] == '/' {
		sm = sm[1:]
	}
	pos := strings.LastIndex(sm, "/")
	if pos == -1 {
		log.Debugf("bad method name")
		stream.SendError(ctx, fmt.Errorf("malformed method name: %q", stream.Method()))
		return
	}

	service := sm[:pos]
	method := sm[pos+1:]

	s.lock.Lock()

	srv, ok := s.services[service]
	if !ok {
		log.Debugf("unknown service")
		stream.SendError(ctx, fmt.Errorf("unknown service %v", service))
		s.lock.Unlock()
		return
	}

	// Unary RPC or Streaming RPC?
	if md, ok := srv.md[method]; ok {
		log.Debugf("dispatching unary: %s", md.MethodName)
		s.lock.Unlock()
		s.processUnaryRPC(ctx, stream, srv, md)
		return
	}

	if sd, ok := srv.sd[method]; ok {
		log.Debugf("dispatching streaming: %s", sd.StreamName)
		s.lock.Unlock()
		s.processStreamingRPC(ctx, stream, srv, sd)
		return
	}

	log.Debugf("unknown method: %s", method)
	stream.SendError(ctx, fmt.Errorf("unknown method %v", method))
	s.lock.Unlock()
}

type Unmarshaler interface {
	Unmarshal(req []byte) error
}

func (s *Server) processUnaryRPC(ctx context.Context, stream Stream, srv *service, md *MethodDesc) (err error) {
	req, err := stream.RecvData(ctx, math.MaxInt32)

	if err != nil {
		log.Debugf("recvMsg error: %s", err)

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

	log.Debugf("received request: %d", len(req))

	pbUnmarshal := func(v interface{}) error {
		if m, ok := v.(Unmarshaler); ok {
			err := m.Unmarshal(req)
			if err != nil {
				log.Debugf("Unable to unmarshal %d for %T", crc32.ChecksumIEEE(req), v)
			}
			return err
		}

		return fmt.Errorf("Invalid request type: %T", v)
	}

	reply, appErr := md.Handler(srv.server, ctx, pbUnmarshal, nil)
	if appErr != nil {
		log.Debugf("handler reported error for %s: %s", md.MethodName, appErr)
		err = stream.SendError(ctx, appErr)
		if appErr != nil {
			log.Printf("mesh-grpc: Error sending err back to client: %s", err)
		}

		return appErr
	}

	return stream.SendMsg(ctx, reply)
}

type contextStream struct {
	ctx    context.Context
	stream Stream
}

func (s *contextStream) RecvMsg(reply interface{}) error {
	return s.stream.RecvMsg(s.ctx, reply)
}

func (s *contextStream) SendMsg(reply interface{}) error {
	return s.stream.SendMsg(s.ctx, reply)
}

func (s *Server) processStreamingRPC(ctx context.Context, stream Stream, srv *service, sd *StreamDesc) (err error) {
	appErr := sd.Handler(srv.server, &contextStream{ctx, stream})
	if appErr != nil {
		err = stream.SendError(ctx, appErr)
		if appErr != nil {
			log.Printf("mesh-grpc: Error sending err back to client: %s", err)
		}

		return appErr
	}

	return nil
}
