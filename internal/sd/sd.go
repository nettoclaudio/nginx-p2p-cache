package sd

import "context"

type ServiceDiscoverer interface {
	Discover(ctx context.Context, peers chan<- string) error
}
