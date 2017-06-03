package crypto

import (
	"github.com/evanphx/mesh"
	"github.com/flynn/noise"
)

type CipherState interface {
	Decrypt(out, ad, cipher []byte) ([]byte, error)
	Encrypt(out, ad, cipher []byte) []byte
}

type KKInitState struct {
	hs *noise.HandshakeState
}

func NewKKInitiator(key DHKey, peer mesh.Identity) *KKInitState {
	hs := noise.NewHandshakeState(noise.Config{
		CipherSuite:   CipherSuite,
		Random:        RNG,
		Pattern:       noise.HandshakeKK,
		Initiator:     true,
		StaticKeypair: noise.DHKey(key),
		PeerStatic:    peer,
	})

	return &KKInitState{hs}
}

func (k *KKInitState) Start(out, payload []byte) []byte {
	o, _, _ := k.hs.WriteMessage(out, payload)
	return o
}

func (k *KKInitState) Finish(out, message []byte) ([]byte, CipherState, CipherState, error) {
	return k.hs.ReadMessage(out, message)
}

type KKResponderState struct {
	hs *noise.HandshakeState
}

func NewKKResponder(key DHKey, peer mesh.Identity) *KKResponderState {
	hs := noise.NewHandshakeState(noise.Config{
		CipherSuite:   CipherSuite,
		Random:        RNG,
		Pattern:       noise.HandshakeKK,
		StaticKeypair: noise.DHKey(key),
		PeerStatic:    peer,
	})

	return &KKResponderState{hs}
}

func (k *KKResponderState) Start(out, message []byte) ([]byte, error) {
	out, _, _, err := k.hs.ReadMessage(out, message)
	return out, err
}

func (k *KKResponderState) Finish(out, payload []byte) ([]byte, CipherState, CipherState) {
	return k.hs.WriteMessage(out, payload)
}
