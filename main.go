package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/nettoclaudio/nginx-p2p-cache/internal/nginx"
	cr "github.com/nettoclaudio/nginx-p2p-cache/internal/nginx/cache_repository"
	pb "github.com/nettoclaudio/nginx-p2p-cache/internal/nginx/cache_repository/v1"
	"github.com/nettoclaudio/nginx-p2p-cache/internal/sd"
)

var cfg struct {
	CacheDir                         string
	ServiceDiscoveryMethod           string
	ServiceDiscoveryDNS              string
	ServiceDiscoveryDNSQueryInterval time.Duration
	Port                             int
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
	flag.IntVar(&cfg.Port, "port", 8000, "Server TCP port")
	flag.Parse()

	logger := zap.Must(zap.NewProduction())
	if cfg.Debug {
		logger = zap.Must(zap.NewDevelopment())
	}

	address := fmt.Sprintf(":%d", cfg.Port)

	l, err := net.Listen("tcp", address)
	if err != nil {
		logger.Fatal("Failed to listen on TCP port", zap.String("address", address), zap.Error(err))
	}
	defer l.Close()

	logger.Info("Starting the Nginx P2P cache sidecar")
	defer logger.Info("Finishing Nginx P2P cache sidecar")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eg, egctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		select {
		case <-stop:
			logger.Info("Received a termination signal...")
			cancel()
		}
		return nil
	})

	watcher := &cr.CacheWatcher{
		Directory: cfg.CacheDir,
		Logger:    logger,
	}

	eg.Go(func() error { return watcher.Watch(egctx) })

	s := grpc.NewServer()
	pb.RegisterCacheRepositoryServer(s, &pb.Server{Logger: logger, Cache: watcher})

	eg.Go(func() error {
		<-ctx.Done()
		logger.Info("Finishing web server...")
		s.GracefulStop()
		return nil
	})

	eg.Go(func() error {
		logger.Info("Starting gRPC server", zap.String("address", l.Addr().String()))
		return s.Serve(l)
	})

	eg.Go(func() error {
		cm := &nginx.CacheManager{
			Discoverer: &sd.DNSServiceDiscovery{
				Domain:      cfg.ServiceDiscoveryDNS,
				Interval:    cfg.ServiceDiscoveryDNSQueryInterval,
				DisableIPv6: cfg.ServiceDiscoveryDNSDisableIPv6,
				Logger:      logger,
			},
			Watcher:  watcher,
			Interval: time.Minute,
			Logger:   logger,
			Port:     cfg.Port,
		}
		return cm.Reconcile(egctx)
	})

	if err := eg.Wait(); err != nil {
		logger.Fatal("Something went wrong :(", zap.Error(err))
	}
}
