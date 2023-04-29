package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
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

	fmt.Println("Hello world!")

	<-stop
	fmt.Println("Received a signal to stop the service...")
}
