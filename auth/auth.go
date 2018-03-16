package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"

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

func (a *Authorizer) Sign(msg []byte) ([]byte, []byte) {
	return a.publicKey, ed25519.Sign(a.privateKey, msg)
}

func (a *Authorizer) Verify(msg, sig []byte) bool {
	return ed25519.Verify(a.publicKey, msg, sig)
}

func (a *Authorizer) VerifySigner(signer, signature, msg []byte) bool {
	if !bytes.Equal(a.publicKey, signer) {
		return false
	}

	return ed25519.Verify(a.publicKey, msg, signature)
}

type exportedAuthorizer struct {
	Network string `json:"network"`
	Public  string `json:"public"`
	Private string `json:"private"`
}

func (a *Authorizer) SaveToPath(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	defer f.Close()

	x := exportedAuthorizer{
		Network: a.name,
		Public:  hex.EncodeToString(a.publicKey),
		Private: hex.EncodeToString(a.privateKey),
	}

	return json.NewEncoder(f).Encode(x)
}

func ReadFromPath(path string) (*Authorizer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	x := exportedAuthorizer{}

	var a Authorizer

	err = json.NewDecoder(f).Decode(&x)
	if err != nil {
		return nil, err
	}

	a.name = x.Network
	a.publicKey, err = hex.DecodeString(x.Public)
	if err != nil {
		return nil, err
	}

	a.privateKey, err = hex.DecodeString(x.Private)
	if err != nil {
		return nil, err
	}

	return &a, nil
}
