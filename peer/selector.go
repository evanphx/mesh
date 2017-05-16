package peer

import (
	"errors"
	"time"
)

type PipeSelector struct {
	Pipe      string
	Subsystem string
	Tags      map[string]string
}

var ErrNoMatch = errors.New("no matching advertisements")

func (p *Peer) lookupAdver(sel *PipeSelector) (*localAdver, error) {
	p.adverLock.Lock()
	defer p.adverLock.Unlock()

	now := time.Now()

	for _, ld := range p.advers {
		if !ld.valid(now) {
			continue
		}

		if ld.adver.matchSelector(sel) {
			if p.reachable(ld.adver.Owner) {
				return ld, nil
			}
		}
	}

	return nil, ErrNoMatch
}

func (p *Peer) lookupSelector(sel *PipeSelector) (Identity, string, error) {
	ld, err := p.lookupAdver(sel)
	if err != nil {
		return nil, "", err
	}

	return ld.adver.Owner, ld.adver.Pipe, nil
}

func (a *Advertisement) matchSelector(sel *PipeSelector) bool {
	switch {
	case sel.Pipe != "":
		if a.Pipe != sel.Pipe {
			return false
		}
	case sel.Subsystem != "":
		if a.Subsystem != sel.Subsystem {
			return false
		}
	default:
		return false
	}

	if sel.Tags != nil {
		if a.Tags == nil {
			return false
		}

		for k, v := range sel.Tags {
			if a.Tags[k] != v {
				return false
			}
		}
	}

	return true
}
