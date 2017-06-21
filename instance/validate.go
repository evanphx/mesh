package instance

func (i *Instance) CredsFor(name string) []byte {
	return nil
}

func (i *Instance) Validate(name string, a, b []byte) bool {
	return true
}
