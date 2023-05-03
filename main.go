package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nettoclaudio/nginx-p2p-cache/internal/nginx"
)

var cfg struct {
	CacheDir                         string
	ServiceDiscoveryMethod           string
	ServiceDiscoveryDNS              string
	ServiceDiscoveryDNSQueryInterval time.Duration
}

func main() {
	flag.StringVar(&cfg.CacheDir, "cache-dir", "", "Nginx cache directory")
	flag.StringVar(&cfg.ServiceDiscoveryMethod, "service-discovery-method", "dns", "Method used to discover peers (allowed methods are: \"dns\")")
	flag.StringVar(&cfg.ServiceDiscoveryDNS, "service-discovery-dns", "", "Domain name used to discover peers")
	flag.DurationVar(&cfg.ServiceDiscoveryDNSQueryInterval, "service-discovery-dns-query-interval", time.Second, "Interval between consecutive DNS queries")
	flag.Parse()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Starting Nginx P2P cache sidecar...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cw := &nginx.CacheWatcher{}

	go func() {
		if err := cw.Watch(ctx, cfg.CacheDir); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to watch cache files:", err)
			cancel()
		}
	}()

	go func() {
		for added := range cw.Added() {
			fmt.Println("[ADDED]: ", added)
		}
	}()

	go func() {
		for removed := range cw.Removed() {
			fmt.Println("[REMOVED]: ", removed)
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)

		for {
			select {
			case <-ticker.C:
				fmt.Printf("Keys: %+v\n", cw.Keys())

			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case <-stop:
		fmt.Println("Received a signal to stop the service...")

	case <-ctx.Done():
		fmt.Println("Something went wrong... aborting program execution")
		os.Exit(1)
	}
}
