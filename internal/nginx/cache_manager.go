package nginx

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	cr "github.com/nettoclaudio/nginx-p2p-cache/internal/nginx/cache_repository"
	crv1 "github.com/nettoclaudio/nginx-p2p-cache/internal/nginx/cache_repository/v1"
	"github.com/nettoclaudio/nginx-p2p-cache/internal/sd"
)

type CacheManager struct {
	Discoverer sd.ServiceDiscoverer
	Watcher    *cr.CacheWatcher
	Interval   time.Duration
	Logger     *zap.Logger
	Port       int

	peers sync.Map // grpc connection by peer (IP)
}

func (cm *CacheManager) Reconcile(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if cm.Logger == nil {
		cm.Logger = zap.NewNop()
	}

	cm.Logger.Debug("Starting cache manager")
	defer cm.Logger.Debug("Finishing cache manager")

	eg, egctx := errgroup.WithContext(ctx)

	// Managing peer connections
	defer cm.closeConnections()

	eg.Go(func() error { cm.handlePeers(egctx); return nil })
	eg.Go(func() error { return cm.Discoverer.Discover(egctx) })

	eg.Go(func() error { return cm.reconcile(egctx) })

	return eg.Wait()
}

func (cm *CacheManager) reconcile(ctx context.Context) error {
	ticker := time.NewTicker(cm.Interval)

	for {
		select {
		case <-ticker.C:

			cm.peers.Range(func(key, value any) bool {
				peer := key.(string)
				cm.Logger.Debug("Calling RPC server", zap.String("peer", peer))

				conn := value.(*grpc.ClientConn)
				r, err := crv1.NewCacheRepositoryClient(conn).List(context.TODO(), &crv1.ListRequest{})
				if err != nil {
					cm.Logger.Error("failed to list cache", zap.String("peer", peer), zap.Error(err))
					return true
				}

				cm.Logger.Info("Response", zap.Any("response", r))

				return true
			})

		case <-ctx.Done():
			return nil
		}
	}
}

func (cm *CacheManager) handlePeers(ctx context.Context) {
	for {
		select {
		case peer, isOpen := <-cm.Discoverer.Added():
			if !isOpen {
				cm.Logger.Error("Channel of added peers is closed")
				break
			}

			cm.addedPeer(peer)

		case peer, isOpen := <-cm.Discoverer.Removed():
			if !isOpen {
				cm.Logger.Error("Channel of removed peers is closed")
				break
			}

			cm.removedPeer(peer)

		case <-ctx.Done():
			return
		}
	}
}

func (cm *CacheManager) addedPeer(peer string) {
	_, ok := cm.peers.Load(peer)
	if ok { // do nothing
		return
	}

	conn, err := cm.dial(peer)
	if err != nil {
		cm.Logger.Error("Failed to open connection", zap.String("peer", peer), zap.Error(err))
		return
	}

	cm.peers.Store(peer, conn)
}

func (cm *CacheManager) removedPeer(peer string) {
	value, ok := cm.peers.Load(peer)
	if !ok {
		return
	}

	conn, ok := value.(*grpc.ClientConn)
	if !ok {
		return
	}

	if err := conn.Close(); err != nil {
		cm.Logger.Error("Failed to close connection", zap.String("peer", peer), zap.Error(err))
	}

	cm.peers.Delete(peer)
}

func (cm *CacheManager) dial(address string) (conn *grpc.ClientConn, err error) {
	target := fmt.Sprintf("dns:///%s:%d", address, cm.Port)

	maxRetries := 20

	for i := 0; i < maxRetries; i++ {
		cm.Logger.Debug("Dialing to address", zap.String("target", target), zap.Int("attempt", i+1))

		conn, err = grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			return
		}

		t := time.Duration(math.Pow(2, float64(i)))
		time.Sleep(t * time.Second)
	}

	return
}

func (cm *CacheManager) closeConnections() {
	cm.peers.Range(func(key, _ any) bool {
		peer, ok := key.(string)
		if !ok {
			return true
		}

		cm.removedPeer(peer)

		return true
	})
}
