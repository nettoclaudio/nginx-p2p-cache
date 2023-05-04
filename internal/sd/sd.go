package sd

import "context"

type ServiceDiscoverer interface {
	Added() <-chan string
	Removed() <-chan string
	Discover(ctx context.Context) error
}
