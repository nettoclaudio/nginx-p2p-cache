package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/nettoclaudio/nginx-p2p-cache/internal/nginx"
	"github.com/nettoclaudio/nginx-p2p-cache/internal/sd"
)

var cfg struct {
	CacheDir                         string
	ServiceDiscoveryMethod           string
	ServiceDiscoveryDNS              string
	ServiceDiscoveryDNSQueryInterval time.Duration
	ServiceDiscoveryDNSDisableIPv6   bool
	Debug                            bool
}

func main() {
	flag.StringVar(&cfg.CacheDir, "cache-dir", "", "Nginx cache directory")
	flag.StringVar(&cfg.ServiceDiscoveryMethod, "service-discovery-method", "dns", "Method used to discover peers (allowed methods are: \"dns\")")
	flag.StringVar(&cfg.ServiceDiscoveryDNS, "service-discovery-dns", "", "Domain name used to discover peers")
	flag.DurationVar(&cfg.ServiceDiscoveryDNSQueryInterval, "service-discovery-dns-query-interval", time.Second, "Interval between consecutive DNS queries")
	flag.BoolVar(&cfg.ServiceDiscoveryDNSDisableIPv6, "service-discovery-dns-disable-ipv6", false, "Whether should disable AAAA queries")
	flag.BoolVar(&cfg.Debug, "debug", false, "Whether should run in debug mode")
	flag.Parse()

	logger := zap.Must(zap.NewProduction())
	if cfg.Debug {
		logger = zap.Must(zap.NewDevelopment())
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	logger.Info("Starting the Nginx P2P cache sidecar")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		defer cancel()

		cm := &nginx.CacheManager{
			Discoverer: &sd.DNSServiceDiscovery{
				Domain:      cfg.ServiceDiscoveryDNS,
				Interval:    cfg.ServiceDiscoveryDNSQueryInterval,
				DisableIPv6: cfg.ServiceDiscoveryDNSDisableIPv6,
				Logger:      logger,
			},
			Watcher: &nginx.CacheWatcher{
				Directory: cfg.CacheDir,
				Logger:    logger,
			},
			Interval: time.Minute,
			Logger:   logger,
		}

		if err := cm.Reconcile(ctx); err != nil {
			logger.Error("Failed to start cache manager", zap.Error(err))
		}
	}()

	select {
	case <-stop:
		logger.Debug("Received a stop signal from system")

	case <-ctx.Done():
		logger.Fatal("Sometheing went wrong on program execution... aborting.")
	}
}
