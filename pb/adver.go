package pb

import (
	"time"

	"github.com/evanphx/mesh"
)

const DefaultTTL = 300

func (a *Advertisement) ExpiresAt() time.Time {
	ttl := a.TimeToLive
	if a.TimeToLive == 0 {
		ttl = DefaultTTL
	}

	return time.Now().Add(time.Duration(ttl) * time.Second)
}

type Signer interface {
	Sign(msg []byte) (signer []byte, signature []byte)
}

type Verifier interface {
	VerifySigner(signer, signature, msg []byte) bool
}

func (a *Advertisement) Sign(s Signer) {
	a.Signer = nil
	a.Signature = nil

	data, err := a.Marshal()
	if err != nil {
		panic(err)
	}

	a.Signer, a.Signature = s.Sign(data)
}

func (a *Advertisement) Verify(v Verifier) bool {
	var cp Advertisement = *a

	cp.Signer = nil
	cp.Signature = nil

	data, err := cp.Marshal()
	if err != nil {
		return false
	}

	return v.VerifySigner(a.Signer, a.Signature, data)
}

func (a *Advertisement) MatchSelector(sel *mesh.PipeSelector) bool {
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
