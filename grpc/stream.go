package grpc

import (
	"context"
	"errors"
	"math"
)

type Stream interface {
	RecvMsg(ctx context.Context, max int) ([]byte, error)
	SendError(ctx context.Context, err error) error
	SendReply(ctx context.Context, v interface{}) error

	SendRequest(ctx context.Context, args interface{}) error
	ReadReply(ctx context.Context, reply interface{}) error
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

type Unmarshaler interface {
	Unmarshal(msg []byte) error
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
