package ping

import (
	"context"
	"sync"
	"time"

	"github.com/evanphx/mesh"
	"github.com/evanphx/mesh/log"
	"github.com/evanphx/mesh/pb"
)

type Handler struct {
	proto    int32
	sender   mesh.Sender
	lock     sync.Mutex
	nextSess uint64
	pings    map[uint64]chan struct{}
}

func (h *Handler) Setup(proto int32, sender mesh.Sender) {
	h.pings = make(map[uint64]chan struct{})
	h.proto = proto
	h.sender = sender
}

func (h *Handler) Handle(ctx context.Context, hdr *pb.Header) error {
	var msg Message

	err := msg.Unmarshal(hdr.Body)
	if err != nil {
		return err
	}

	if !msg.Reply {
		// Send back a pong
		msg.Reply = true

		return h.sender.SendData(ctx, hdr.Sender, h.proto, &msg)
	}

	h.lock.Lock()

	c, ok := h.pings[msg.Session]

	h.lock.Unlock()

	if ok {
		select {
		case c <- struct{}{}:
			// all good
		default:
			// omg super fast response
			go func() {
				c <- struct{}{}
			}()
		}
	}

	return nil
}

func (h *Handler) Ping(ctx context.Context, id mesh.Identity) (time.Duration, error) {
	h.lock.Lock()

	sess := h.nextSess
	h.nextSess++

	c := make(chan struct{})

	h.pings[sess] = c

	h.lock.Unlock()

	var msg Message
	msg.Session = sess

	sent := time.Now()

	err := h.sender.SendData(ctx, id, h.proto, &msg)
	if err != nil {
		return 0, err
	}

	var ret time.Duration

	select {
	case <-c:
		ret = time.Since(sent)
		err = nil
	case <-ctx.Done():
		log.Debugf("ping wait canceled")
		err = ctx.Err()
	}

	h.lock.Lock()
	delete(h.pings, sess)
	h.lock.Unlock()

	// Do a quick drain of c in case the ping came in after we
	// timed out but before we got to the lock.
	select {
	case <-c:
	default:
	}

	return ret, err
}
