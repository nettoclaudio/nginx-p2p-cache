package sd_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/nettoclaudio/nginx-p2p-cache/internal/sd"
)

func TestDNSServiceDiscovery_Discover_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	err := NewServiceDiscoveryDNS("my-sd-domain.examle.com").Discover(ctx, nil)
	assert.Error(t, err)
	assert.EqualError(t, err, "context canceled")
}

func TestDNSServiceDiscovery_Discover(t *testing.T) {
	peers := make(chan string)
	defer close(peers)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	go func() {
		err := NewServiceDiscoveryDNS("my-sd-domain.169-196-255-255.nip.io").Discover(ctx, peers)
		assert.NoError(t, err)
	}()

	assert.Equal(t, "169.196.255.255", <-peers)
}
