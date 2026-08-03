package schema

import (
	"testing"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/stretchr/testify/require"
)

func TestRbacPolicyRevisionUsesRevisionIdentity(t *testing.T) {
	fields := fieldDescriptors(RbacPolicyRevision{}.Fields())

	require.Equal(t, "revision", fields["id"].StorageKey)
	require.True(t, fields["id"].Immutable)
	require.True(t, fields["role_id"].Optional)
	require.True(t, fields["user_id"].Optional)
	require.True(t, fields["permission_id"].Optional)
}

func TestRbacPolicyOutboxDefaultsAndConstraints(t *testing.T) {
	fields := fieldDescriptors(RbacPolicyOutboxEvent{}.Fields())

	require.Equal(t, defaultRbacPolicyOutboxStatus, fields["status"].Default)
	require.Equal(t, 0, fields["attempt_count"].Default)
	require.True(t, fields["revision"].Unique)
	require.True(t, fields["event_id"].Unique)
	require.True(t, fields["idempotency_key"].Unique)
	require.True(t, fields["claim_token"].Optional)
	require.True(t, fields["claim_token"].Nillable)
	require.True(t, fields["claimed_until"].Optional)
	require.True(t, fields["claimed_until"].Nillable)
	require.True(t, fields["delivered_at"].Optional)

	for _, status := range []string{
		rbacPolicyOutboxStatusPending,
		rbacPolicyOutboxStatusProcessing,
		rbacPolicyOutboxStatusFailed,
		rbacPolicyOutboxStatusDelivered,
	} {
		for _, validator := range fields["status"].Validators {
			require.NoError(t, validator.(func(string) error)(status))
		}
	}
	var validationErr error
	for _, validator := range fields["status"].Validators {
		if err := validator.(func(string) error)("unknown"); err != nil {
			validationErr = err
			break
		}
	}
	require.Error(t, validationErr)
}

func fieldDescriptors(fields []ent.Field) map[string]*field.Descriptor {
	descriptors := make(map[string]*field.Descriptor, len(fields))
	for _, schemaField := range fields {
		descriptor := schemaField.Descriptor()
		descriptors[descriptor.Name] = descriptor
	}
	return descriptors
}
