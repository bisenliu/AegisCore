package query

import (
	"context"
	"sort"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// RouteDiffResult 描述已发现路由和权限目录之间的差异。
type RouteDiffResult struct {
	MissingInPermissions []permissionapplication.DiscoveredRoute
	StalePermissions     []permissiondomain.Permission
}

// GetRouteDiff 返回路由发现和权限目录之间的只读差异。
func (s *permissionQueryService) GetRouteDiff(ctx context.Context) (*RouteDiffResult, error) {
	discovered, err := s.scanner.DiscoverRoutes(ctx)
	if err != nil {
		return nil, err
	}
	permissions, err := s.store.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	permissionSet := make(map[string]struct{}, len(permissions))
	for i := range permissions {
		permissionSet[permissions[i].Identity().Key()] = struct{}{}
	}
	discoveredSet := make(map[string]struct{}, len(discovered))
	missing := make([]permissionapplication.DiscoveredRoute, 0)
	for i := range discovered {
		identity, err := permissiondomain.NewRouteIdentity(discovered[i].Method, discovered[i].Path)
		if err != nil {
			return nil, err
		}
		discovered[i].Method = identity.Method
		discovered[i].Path = identity.PathTemplate
		discoveredSet[identity.Key()] = struct{}{}
		if _, ok := permissionSet[identity.Key()]; !ok {
			missing = append(missing, discovered[i])
		}
	}

	stale := make([]permissiondomain.Permission, 0)
	for i := range permissions {
		if _, ok := discoveredSet[permissions[i].Identity().Key()]; !ok {
			stale = append(stale, permissions[i])
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i].Method+missing[i].Path < missing[j].Method+missing[j].Path })
	sort.Slice(stale, func(i, j int) bool {
		return stale[i].HTTPMethod+stale[i].PathTemplate < stale[j].HTTPMethod+stale[j].PathTemplate
	})
	return &RouteDiffResult{MissingInPermissions: missing, StalePermissions: stale}, nil
}
