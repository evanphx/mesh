package instance

import (
	"context"
	"time"

	"github.com/evanphx/mesh"
)

func (i *Instance) Ping(ctx context.Context, id mesh.Identity) (time.Duration, error) {
	return i.pings.Ping(ctx, id)
}
