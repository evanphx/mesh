package grpc

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/evanphx/mesh/log"
)

type Stream interface {
	Method() string

	RecvMsg(ctx context.Context, max int) ([]byte, error)
	SendError(ctx context.Context, err error) error
	SendReply(ctx context.Context, v interface{}) error

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
	log.Debugf("sending error back: %s", err)

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

func (b *ByteStream) SendReply(ctx context.Context, v interface{}) error {
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
	frame *Frame
	tr    Transport
}

func (s *serverStream) RecvMsg(ctx context.Context, max int) ([]byte, error) {
	return s.frame.Body, nil
}

func (s *serverStream) SendError(ctx context.Context, err error) error {
	log.Debugf("sending error back: %s", err)

	var frame Frame

	frame.Type = ERROR
	frame.Body = []byte(err.Error())

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return s.tr.SendFinal(ctx, data)
}

func (s *serverStream) SendReply(ctx context.Context, v interface{}) error {
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

func (s *serverStream) Method() string {
	return s.frame.Method
}

func (s *serverStream) Close(ctx context.Context) error {
	return s.tr.Close(ctx)
}

type clientStream struct {
	tr     Transport
	method string
}

func (s *clientStream) SendRequest(ctx context.Context, args interface{}) error {
	m, ok := args.(Marshaler)
	if !ok {
		return ErrInvalidValue
	}

	body, err := m.Marshal()
	if err != nil {
		return err
	}

	var frame Frame

	frame.Type = REQUEST
	frame.Method = s.method
	frame.Body = body

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	return s.tr.Send(ctx, data)
}

func (s *clientStream) ReadReply(ctx context.Context, reply interface{}) error {
	data, err := s.tr.Recv(ctx)
	if err != nil {
		return err
	}

	var frame Frame

	err = frame.Unmarshal(data)
	if err != nil {
		return err
	}

	if frame.Type == ERROR {
		return fmt.Errorf("grpc remote error: %s", string(frame.Body))
	}

	if frame.Type != REPLY {
		return fmt.Errorf("Invalid protocol state, expected REPLY, got %s", frame.Type)
	}

	u, ok := reply.(Unmarshaler)
	if !ok {
		return ErrInvalidValue
	}

	return u.Unmarshal(frame.Body)
}

func (s *clientStream) Close(ctx context.Context) error {
	return s.tr.Close(ctx)
}
