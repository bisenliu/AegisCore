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
	require.True(t, fields["permission_id"].Optional)
}

func TestRbacUserRoleRevisionUsesRevisionIdentity(t *testing.T) {
	fields := fieldDescriptors(RbacUserRoleRevision{}.Fields())

	require.Equal(t, "revision", fields["id"].StorageKey)
	require.True(t, fields["id"].Immutable)
	require.False(t, fields["user_id"].Optional)
	require.True(t, fields["role_id"].Optional)
}

func TestTimestampMixinsUseMillisDefaults(t *testing.T) {
	createdAt := createdAtMillisMixin{}.Fields()[0].Descriptor()
	updatedAt := updatedAtMillisMixin{}.Fields()[0].Descriptor()

	require.Equal(t, "created_at", createdAt.Name)
	require.True(t, createdAt.Immutable)
	require.Equal(t, "创建时间戳毫秒", createdAt.Comment)
	require.NotNil(t, createdAt.Default)
	require.IsType(t, func() int64 { return 0 }, createdAt.Default)

	require.Equal(t, "updated_at", updatedAt.Name)
	require.False(t, updatedAt.Immutable)
	require.Equal(t, "更新时间戳毫秒", updatedAt.Comment)
	require.NotNil(t, updatedAt.Default)
	require.NotNil(t, updatedAt.UpdateDefault)
	require.IsType(t, func() int64 { return 0 }, updatedAt.Default)
	require.IsType(t, func() int64 { return 0 }, updatedAt.UpdateDefault)
}

func fieldDescriptors(fields []ent.Field) map[string]*field.Descriptor {
	descriptors := make(map[string]*field.Descriptor, len(fields))
	for _, schemaField := range fields {
		descriptor := schemaField.Descriptor()
		descriptors[descriptor.Name] = descriptor
	}
	return descriptors
}
