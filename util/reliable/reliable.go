package reliable

import (
	"sync"
)

type SeqNum uint64

type WindowTracker struct {
	lock      sync.Mutex
	seqNo     SeqNum
	threshold SeqNum
	slots     [][]byte

	maxWindow int
	curWindow int
}

const (
	QueueSlots = 10
	OneMB      = 1024 * 1024
)

type Config struct {
	WindowSlots int
	WindowBytes int
}

func NewWindowTracker(cfg Config) *WindowTracker {
	return &WindowTracker{
		slots:     make([][]byte, cfg.WindowSlots),
		curWindow: cfg.WindowBytes,
		maxWindow: cfg.WindowBytes,
	}
}

func (s SeqNum) After(i SeqNum) bool {
	return s > i
}

func (r *WindowTracker) slotNumber(s SeqNum) int {
	return int(s - r.threshold - 1)
}

func (r *WindowTracker) Queue(b []byte) (SeqNum, bool) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if len(b) > r.curWindow {
		return 0, false
	}

	var (
		next = r.seqNo + 1
		slot = r.slotNumber(next)
	)

	if slot >= len(r.slots) {
		return 0, false
	}

	r.seqNo = next

	r.slots[r.slotNumber(next)] = b

	r.curWindow -= len(b)

	return next, true
}

func (r *WindowTracker) Ack(n SeqNum) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if n < r.threshold {
		return nil
	}

	var (
		start = r.slotNumber(r.threshold) + 1
		fin   = r.slotNumber(n)
	)

	for i := start; i <= fin; i++ {
		r.curWindow += len(r.slots[i])
		r.slots[i] = nil
	}

	r.threshold = n

	return nil
}

func (r *WindowTracker) Retrieve(n SeqNum) ([]byte, bool) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if n <= r.threshold {
		return nil, false
	}

	slot := r.slotNumber(n)
	if slot >= len(r.slots) {
		return nil, false
	}

	return r.slots[slot], true
}

func (r *WindowTracker) Outstanding() (SeqNum, SeqNum, bool) {
	if r.seqNo == r.threshold {
		return 0, 0, false
	}

	return r.threshold + 1, r.seqNo, true
}
