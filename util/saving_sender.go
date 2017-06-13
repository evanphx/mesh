package util

import (
	"context"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/pb"
)

type SentMessage struct {
	Dest    mesh.Identity
	Proto   int32
	Message interface{}
}

func (s *SentMessage) ToHeader(sdr mesh.Identity) *pb.Header {
	var hdr pb.Header
	hdr.Sender = sdr
	hdr.Destination = s.Dest
	hdr.Proto = s.Proto

	data, err := s.Message.(mesh.Marshaler).Marshal()
	if err != nil {
		panic(err)
	}

	hdr.Body = data

	return &hdr
}

type SavingSender struct {
	Messages []*SentMessage
}

func (ss *SavingSender) SendData(ctx context.Context, id mesh.Identity, proto int32, msg interface{}) error {
	ss.Messages = append(ss.Messages, &SentMessage{
		Dest:    id,
		Proto:   proto,
		Message: msg,
	})

	return nil
}
