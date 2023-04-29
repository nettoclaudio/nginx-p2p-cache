package sd

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"
)

type DNSSDOptions struct {
	Interval time.Duration
}

var DefaultDNSSDOptions = DNSSDOptions{
	Interval: (10 * time.Second),
}

func NewServiceDiscoveryDNS(domain string) *DNSServiceDiscovery {
	return NewServiceDiscoveryDNSWithOptions(domain, DefaultDNSSDOptions)
}

func NewServiceDiscoveryDNSWithOptions(domain string, opts DNSSDOptions) *DNSServiceDiscovery {
	return &DNSServiceDiscovery{
		Domain:   domain,
		Interval: opts.Interval,
	}
}

var _ ServiceDiscoverer = (*DNSServiceDiscovery)(nil)

type DNSServiceDiscovery struct {
	Domain   string
	Interval time.Duration
}

func (dsd *DNSServiceDiscovery) Discover(ctx context.Context, peers chan<- string) error {
	if err := ctx.Err(); err != nil { // e.g. context deadline exceeded
		return err
	}

	// NOTE: running the first discover call outside of loop to catch and return possible DNS resolution errors.
	if err := discoverPeersByDomain(dsd.Domain, peers); err != nil {
		return err
	}

	tc := time.NewTicker(dsd.Interval)
	defer tc.Stop()

	for {
		select {
		case <-tc.C:
			if err := discoverPeersByDomain(dsd.Domain, peers); err != nil {
				fmt.Fprintln(os.Stderr, "Failed to discover peers: ", err)
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func discoverPeersByDomain(domain string, peers chan<- string) error {
	fmt.Printf("Running service discovery by looking peers from %q domain\n", domain)

	ips, err := net.LookupIP(domain)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d peers: %v\n", len(ips), ips)

	for _, ip := range ips {
		peers <- ip.String()
	}

	return nil
}
