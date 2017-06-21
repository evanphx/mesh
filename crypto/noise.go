package crypto

import (
	"encoding/hex"

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

type XXResponderState struct {
	hs *noise.HandshakeState
}

func NewXXResponder(key DHKey) *XXResponderState {
	hs := noise.NewHandshakeState(noise.Config{
		CipherSuite:   CipherSuite,
		Random:        RNG,
		Pattern:       noise.HandshakeXX,
		StaticKeypair: noise.DHKey(key),
		Prologue:      Prologue,
	})

	return &XXResponderState{hs}
}

func (x *XXResponderState) Start(out, message []byte) ([]byte, error) {
	out, _, _, err := x.hs.ReadMessage(out, message)
	return out, err
}

func (x *XXResponderState) Prime(out, message []byte) []byte {
	msg, _, _ := x.hs.WriteMessage(out, message)
	return msg
}

func (x *XXResponderState) Finish(out, message []byte) ([]byte, CipherState, CipherState, error) {
	return x.hs.ReadMessage(out, message)
}

func (x *XXResponderState) Peer() mesh.Identity {
	return mesh.Identity(x.hs.PeerStatic())
}

func (x *XXResponderState) Session() string {
	return hex.EncodeToString(x.hs.ChannelBinding())
}

type XXInitiatorState struct {
	hs *noise.HandshakeState
}

func NewXXInitiator(key DHKey) *XXInitiatorState {
	hs := noise.NewHandshakeState(noise.Config{
		CipherSuite:   CipherSuite,
		Random:        RNG,
		Pattern:       noise.HandshakeXX,
		Initiator:     true,
		StaticKeypair: noise.DHKey(key),
		Prologue:      Prologue,
	})

	return &XXInitiatorState{hs}
}

func (x *XXInitiatorState) Start(out, message []byte) []byte {
	msg, _, _ := x.hs.WriteMessage(out, message)
	return msg
}

func (x *XXInitiatorState) Prime(out, message []byte) ([]byte, error) {
	out, _, _, err := x.hs.ReadMessage(out, message)
	return out, err
}

func (x *XXInitiatorState) Finish(out, message []byte) ([]byte, CipherState, CipherState) {
	return x.hs.WriteMessage(out, message)
}

func (x *XXInitiatorState) Peer() mesh.Identity {
	return mesh.Identity(x.hs.PeerStatic())
}

func (x *XXInitiatorState) Session() string {
	return hex.EncodeToString(x.hs.ChannelBinding())
}
