package peer

type Signer interface {
	Sign(msg []byte) (signer []byte, signature []byte)
}

type Verifier interface {
	VerifySigner(signer, signature, msg []byte) bool
}
