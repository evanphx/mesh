package util

import (
	"context"

	"github.com/evanphx/mesh"
)

type SentMessage struct {
	Dest    mesh.Identity
	Proto   int32
	Message interface{}
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
