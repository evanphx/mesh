package util

import (
	"context"
	"sync"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
)

type HandleHeader interface {
	Handle(context.Context, *pb.Header) error
}

type SendDataer interface {
	SendData(context.Context, mesh.Identity, int32, interface{}) error
}

type SendAdapter struct {
	Sender   mesh.Identity
	Handler  HandleHeader
	Messages []*SentMessage
	Sync     bool

	mu sync.Mutex

	hdrs chan *SentMessage
}

func (s *SendAdapter) SendData(ctx context.Context, dst mesh.Identity, proto int32, msg interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sm := &SentMessage{
		Dest:    dst,
		Proto:   proto,
		Message: msg,
	}

	s.Messages = append(s.Messages, sm)

	if s.Handler == nil {
		panic("NO HANDLER")
	}

	log.Debugf("%s queue for sending to %s", s.Sender.Short(), dst.Short())

	if s.Sync {
		s.Handler.Handle(ctx, sm.ToHeader(s.Sender))
	} else {
		if s.hdrs == nil {
			s.hdrs = make(chan *SentMessage, 10)
			go func() {
				defer log.Debugf("%s handle loop ended", s.Sender.Short())
				for {
					log.Debugf("%s handle loop", s.Sender.Short())
					select {
					case sm := <-s.hdrs:
						log.Debugf("%s handling %v to %s", s.Sender.Short(), sm.Message, dst.Short())
						s.Handler.Handle(ctx, sm.ToHeader(s.Sender))
					case <-ctx.Done():
						return
					}
				}
			}()
		}

		s.hdrs <- sm
	}

	return nil
}

func (s *SendAdapter) At(i int) *SentMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i < 0 {
		i = len(s.Messages) + i
	}

	return s.Messages[i]
}

func (s *SendAdapter) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.Messages)
}
