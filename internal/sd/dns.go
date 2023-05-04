package sd

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

var _ ServiceDiscoverer = (*DNSServiceDiscovery)(nil)

type DNSServiceDiscovery struct {
	Domain      string
	Interval    time.Duration
	Logger      *zap.Logger
	DisableIPv6 bool

	added   chan string
	removed chan string

	data sync.Map
	o    sync.Once
}

func (dsd *DNSServiceDiscovery) Added() <-chan string {
	dsd.startChannels()
	return dsd.added
}

func (dsd *DNSServiceDiscovery) Removed() <-chan string {
	dsd.startChannels()
	return dsd.removed
}

func (dsd *DNSServiceDiscovery) Discover(ctx context.Context) error {
	if err := ctx.Err(); err != nil { // e.g. context deadline exceeded
		return err
	}

	dsd.startChannels()
	defer close(dsd.added)
	defer close(dsd.removed)

	if dsd.Logger == nil {
		dsd.Logger = zap.NewNop()
	}

	dsd.Logger = dsd.Logger.With(zap.String("sd_method", "dns"), zap.String("domain", dsd.Domain), zap.Bool("disable_ipv6", dsd.DisableIPv6))

	dsd.Logger.Debug("Starting service discovery")
	defer dsd.Logger.Debug("Finishing service discovery")

	// NOTE: running the first discover call outside of loop to catch and return possible DNS resolution errors.
	if err := dsd.discoverPeersByDomain(dsd.Domain); err != nil {
		return err
	}

	tc := time.NewTicker(dsd.Interval)
	defer tc.Stop()

	for {
		select {
		case <-tc.C:
			if err := dsd.discoverPeersByDomain(dsd.Domain); err != nil {
				dsd.Logger.Error("Failed to discover peers", zap.Error(err))
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func (dsd *DNSServiceDiscovery) discoverPeersByDomain(domain string) error {
	dsd.Logger.Debug("Looking up peers from DNS")

	network := "ip"
	if dsd.DisableIPv6 {
		network = "ip4"
	}

	ips, err := net.DefaultResolver.LookupIP(context.Background(), network, domain)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			dsd.Logger.Debug("No entries found")
			return nil
		}

		return err
	}

	ipsStr := make([]string, 0, len(ips))
	for _, ip := range ips {
		ipsStr = append(ipsStr, ip.String())
	}

	dsd.Logger.Debug("DNS query finished succesfully", zap.Strings("ips", ipsStr))

	go dsd.notifyChanges(ipsStr)

	return nil
}

func (dsd *DNSServiceDiscovery) notifyChanges(ips []string) {
	observed := make(map[string]struct{})
	for _, ip := range ips {
		observed[ip] = struct{}{}
	}

	actual := make(map[string]struct{})
	dsd.data.Range(func(key, _ any) bool {
		actual[key.(string)] = struct{}{}
		return true
	})

	var toAdd, toRemove []string

	for ip := range observed {
		if _, found := actual[ip]; !found {
			toAdd = append(toAdd, ip)
		}
	}

	for ip := range actual {
		if _, found := observed[ip]; !found {
			toRemove = append(toRemove, ip)
		}
	}

	for _, ip := range toAdd {
		dsd.added <- ip
		dsd.data.Store(ip, struct{}{})
	}

	for _, ip := range toRemove {
		dsd.removed <- ip
		dsd.data.Delete(ip)
	}

}

func (dsd *DNSServiceDiscovery) startChannels() {
	dsd.o.Do(func() { dsd.added, dsd.removed = make(chan string), make(chan string) })
}
