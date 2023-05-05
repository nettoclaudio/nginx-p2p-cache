package v1

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	cr "github.com/nettoclaudio/nginx-p2p-cache/internal/nginx/cache_repository"
)

var _ CacheRepositoryServer = (*Server)(nil)

type Server struct {
	*UnimplementedCacheRepositoryServer
	Cache  *cr.CacheWatcher
	Logger *zap.Logger
}

func (s *Server) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	s.Logger.Debug("List method called")
	defer s.Logger.Debug("List method finished")

	return nil, fmt.Errorf("not implemented yet")
}
