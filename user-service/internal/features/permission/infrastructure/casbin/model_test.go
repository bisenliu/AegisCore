package casbin

import (
	"testing"

	casbinlib "github.com/casbin/casbin/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedModelWildcardMatcher(t *testing.T) {
	model, err := newModel()
	require.NoError(t, err)
	enforcer, err := casbinlib.NewEnforcer(model)
	require.NoError(t, err)
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	_, err = enforcer.AddPolicy(roleSubject(roleID), policyWildcard, policyWildcard)
	require.NoError(t, err)
	allowed, err := enforcer.Enforce(roleSubject(roleID), "/api/v1/users/:id", "DELETE")
	require.NoError(t, err)
	require.True(t, allowed)
}
