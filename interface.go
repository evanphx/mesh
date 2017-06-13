package mesh

import (
	"context"
)

type Marshaler interface {
	Marshal() ([]byte, error)
}

type Sender interface {
	SendData(context.Context, Identity, int32, interface{}) error
}
