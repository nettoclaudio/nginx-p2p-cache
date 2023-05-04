package nginx

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/nettoclaudio/nginx-p2p-cache/internal/sd"
)

type CacheManager struct {
	Discoverer sd.ServiceDiscoverer
	Watcher    *CacheWatcher
	Interval   time.Duration
	Logger     *zap.Logger

	peers sync.Map
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

	go cm.Watcher.Watch(ctx)
	go cm.handleCache(ctx)

	go cm.handlePeers(ctx)
	return cm.Discoverer.Discover(ctx)
}

func (cm *CacheManager) handleCache(ctx context.Context) {
	for {
		select {
		case key, isOpen := <-cm.Watcher.Added():
			if !isOpen {
				return
			}

			cm.Logger.Debug("Key added", zap.String("key", key))

		case key, isOpen := <-cm.Watcher.Removed():
			if !isOpen {
				return
			}

			cm.Logger.Debug("Key removed", zap.String("key", key))

		case <-ctx.Done():
			return
		}
	}
}

func (cm *CacheManager) handlePeers(ctx context.Context) {
	for {
		select {
		case peer, isOpen := <-cm.Discoverer.Added():
			if !isOpen {
				return
			}

			if _, ok := cm.peers.LoadOrStore(peer, struct{}{}); !ok {
				cm.Logger.Debug("New peer added", zap.String("peer", peer))
			}

		case peer, isOpen := <-cm.Discoverer.Removed():
			if !isOpen {
				return
			}

			if _, ok := cm.peers.LoadAndDelete(peer); ok {
				cm.Logger.Debug("Peer deleted", zap.String("peer", peer))
			}

		case <-ctx.Done():
			return
		}
	}
}
