package validators

import (
	"testing"

	"github.com/stretchr/testify/require"

	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestNormalizeRoleFields(t *testing.T) {
	name, description, err := NormalizeRoleFields("  Operator  ", "  Can operate resources  ")

	require.NoError(t, err)
	require.Equal(t, "Operator", name)
	require.Equal(t, "Can operate resources", description)
}

func TestNormalizeRoleFieldsRejectsBlankName(t *testing.T) {
	tests := []string{"", " ", "\t\n"}

	for _, name := range tests {
		t.Run("blank name", func(t *testing.T) {
			gotName, gotDescription, err := NormalizeRoleFields(name, " description ")

			require.ErrorIs(t, err, roledomain.ErrRoleInvalid,
				"err = %v, want ErrRoleInvalid", err)
			require.Empty(t, gotName)
			require.Empty(t, gotDescription)
		})
	}
}
