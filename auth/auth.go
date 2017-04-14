package auth

import (
	"crypto/rand"

	"golang.org/x/crypto/ed25519"
)

type Authorizer struct {
	name       string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

func NewAuthorizer(name string) (*Authorizer, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	auth := &Authorizer{
		name:       name,
		privateKey: priv,
		publicKey:  pub,
	}

	return auth, nil
}

func (a *Authorizer) Name() string {
	return a.name
}

func (a *Authorizer) PublicKey() []byte {
	return a.publicKey
}

func (a *Authorizer) Sign(msg []byte) []byte {
	return ed25519.Sign(a.privateKey, msg)
}

func (a *Authorizer) Verify(msg, sig []byte) bool {
	return ed25519.Verify(a.publicKey, msg, sig)
}
