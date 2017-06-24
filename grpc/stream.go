package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
)

type Stream interface {
	Method() string

	RecvData(ctx context.Context, max int) ([]byte, error)
	RecvMsg(ctx context.Context, m interface{}) error
	SendMsg(ctx context.Context, m interface{}) error
	SendError(ctx context.Context, err error) error

	Close(ctx context.Context) error
}

type VariableByteTransport interface {
	Recv(ctx context.Context, max int) ([]byte, error)
	Send(ctx context.Context, msg []byte) error
}

type ByteStream struct {
	t VariableByteTransport
}

func (b *ByteStream) RecvMsg(ctx context.Context, max int) ([]byte, error) {
	return b.t.Recv(ctx, max)
}

func (b *ByteStream) ReadReply(ctx context.Context, reply interface{}) error {
	data, err := b.t.Recv(ctx, math.MaxInt32)
	if err != nil {
		return err
	}

	u, ok := reply.(Unmarshaler)
	if !ok {
		return ErrInvalidValue
	}

	return u.Unmarshal(data)
}

func (b *ByteStream) SendError(ctx context.Context, err error) error {
	frame := Frame{
		Type: ERROR,
		Body: []byte(err.Error()),
	}

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return b.t.Send(ctx, data)
}

type Marshaler interface {
	Marshal() ([]byte, error)
}

var ErrInvalidValue = errors.New("invalid value type")

func (b *ByteStream) xSendReply(ctx context.Context, v interface{}) error {
	m, ok := v.(Marshaler)
	if !ok {
		return ErrInvalidValue
	}

	body, err := m.Marshal()
	if err != nil {
		return err
	}

	frame := Frame{
		Type: REPLY,
		Body: body,
	}

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return b.t.Send(ctx, data)
}

func (b *ByteStream) SendMsg(ctx context.Context, v interface{}) error {
	m, ok := v.(Marshaler)
	if !ok {
		return ErrInvalidValue
	}

	body, err := m.Marshal()
	if err != nil {
		return err
	}

	frame := Frame{
		Type: DATA,
		Body: body,
	}

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return b.t.Send(ctx, data)
}

func (b *ByteStream) SendRequest(ctx context.Context, v interface{}) error {
	m, ok := v.(Marshaler)
	if !ok {
		return ErrInvalidValue
	}

	body, err := m.Marshal()
	if err != nil {
		return err
	}

	frame := Frame{
		Type: REQUEST,
		Body: body,
	}

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return b.t.Send(ctx, data)
}

type serverStream struct {
	ctx        context.Context
	frame      *Frame
	tr         Transport
	sendClosed bool
}

func (s *serverStream) Context() context.Context {
	return s.ctx
}

func (s *serverStream) RecvData(ctx context.Context, max int) ([]byte, error) {
	if s.sendClosed {
		return nil, io.EOF
	}

	var (
		data []byte
		err  error
	)

	data, err = s.tr.Recv(ctx)
	if err != nil {
		return nil, err
	}

	var frame Frame

	err = frame.Unmarshal(data)
	if err != nil {
		return nil, err
	}

	if frame.Type == ERROR {
		return nil, fmt.Errorf("grpc remote error: %s", string(frame.Body))
	}

	if frame.Type == CLOSE_SEND {
		// TODO do we need to deal with another goroutine also doing a recv and getting stuck
		// waiting?
		s.sendClosed = true
		return nil, io.EOF
	}

	if frame.Type != DATA {
		return nil, fmt.Errorf("Invalid protocol state, expected REPLY, got %s", frame.Type)
	}

	return frame.Body, nil
}

func (s *serverStream) RecvMsg(ctx context.Context, reply interface{}) error {
	data, err := s.RecvData(ctx, math.MaxUint32)
	if err != nil {
		return err
	}

	u, ok := reply.(Unmarshaler)
	if !ok {
		return ErrInvalidValue
	}

	return u.Unmarshal(data)
}

func (s *serverStream) SendError(ctx context.Context, err error) error {
	var frame Frame

	frame.Type = ERROR
	frame.Body = []byte(err.Error())

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return s.tr.SendFinal(ctx, data)
}

func (s *serverStream) xSendReply(ctx context.Context, v interface{}) error {
	m, ok := v.(Marshaler)
	if !ok {
		return ErrInvalidValue
	}

	body, err := m.Marshal()
	if err != nil {
		return err
	}

	var frame Frame

	frame.Type = REPLY
	frame.Body = body

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return s.tr.SendFinal(ctx, data)
}

func (s *serverStream) SendMsg(ctx context.Context, v interface{}) error {
	m, ok := v.(Marshaler)
	if !ok {
		return ErrInvalidValue
	}

	body, err := m.Marshal()
	if err != nil {
		return err
	}

	var frame Frame

	frame.Type = DATA
	frame.Body = body

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return s.tr.Send(ctx, data)
}

func (s *serverStream) Method() string {
	return s.frame.Method
}

func (s *serverStream) Close(ctx context.Context) error {
	var frame Frame

	frame.Type = CLOSE_SEND

	data, err := frame.Marshal()
	if err == nil {
		s.tr.Send(ctx, data)
	}

	return s.tr.Close(ctx)
}

type clientStream struct {
	recvClosed bool
	ctx        context.Context
	tr         Transport
	method     string
}

func (s *clientStream) SendMsg(arg interface{}) error {
	m, ok := arg.(Marshaler)
	if !ok {
		return ErrInvalidValue
	}

	body, err := m.Marshal()
	if err != nil {
		return err
	}

	var frame Frame

	frame.Type = DATA
	frame.Body = body

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return s.tr.Send(s.ctx, data)
}

func (s *clientStream) RecvMsg(reply interface{}) error {
	if s.recvClosed {
		return io.EOF
	}

	data, err := s.tr.Recv(s.ctx)
	if err != nil {
		return err
	}

	var frame Frame

	err = frame.Unmarshal(data)
	if err != nil {
		return err
	}

	switch frame.Type {
	case ERROR:
		return fmt.Errorf("grpc remote error: %s", string(frame.Body))
	case CLOSE_SEND:
		s.recvClosed = true
		s.tr.Close(s.ctx)
		return io.EOF
	case DATA:
		u, ok := reply.(Unmarshaler)
		if !ok {
			return ErrInvalidValue
		}

		return u.Unmarshal(frame.Body)
	default:
		return fmt.Errorf("Invalid protocol state, expected DATA, got %s", frame.Type)
	}
}

func (s *clientStream) SendRequest(ctx context.Context) error {
	var frame Frame

	frame.Type = REQUEST
	frame.Method = s.method

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return s.tr.Send(ctx, data)
}

func (s *clientStream) Close(ctx context.Context) error {
	return s.tr.Close(ctx)
}

func (s *clientStream) CloseSend() error {
	var frame Frame

	frame.Type = CLOSE_SEND
	frame.Method = s.method

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return s.tr.Send(s.ctx, data)
}
