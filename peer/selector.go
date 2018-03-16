package peer

import (
	"errors"
	"time"

	"github.com/evanphx/mesh"
)

var ErrNoMatch = errors.New("no matching advertisements")

func (p *Peer) lookupAdver(sel *mesh.PipeSelector) (*localAdver, error) {
	p.adverLock.Lock()
	defer p.adverLock.Unlock()

	now := time.Now()

	err := ErrNoMatch

	for _, ld := range p.advers {
		if !ld.valid(now) {
			continue
		}

		if ld.adver.MatchSelector(sel) {
			if p.reachable(ld.adver.Owner) {
				return ld, nil
			} else {
				err = ErrUnroutable
			}
		}
	}

	return nil, err
}

func (p *Peer) LookupSelector(sel *mesh.PipeSelector) (mesh.Identity, string, error) {
	ld, err := p.lookupAdver(sel)
	if err != nil {
		return nil, "", err
	}

	return ld.adver.Owner, ld.adver.Pipe, nil
}
