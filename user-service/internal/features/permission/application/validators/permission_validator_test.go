package validators

import (
	"testing"

	"github.com/stretchr/testify/require"

	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestNormalizePermissionFields(t *testing.T) {
	tests := []struct {
		name            string
		permissionName  string
		description     string
		module          string
		method          string
		pathTemplate    string
		wantName        string
		wantDescription string
		wantModule      string
		wantIdentity    permissiondomain.RouteIdentity
	}{
		{
			name:            "trims fields and normalizes route identity",
			permissionName:  "  Read users  ",
			description:     "  List users  ",
			module:          "  user  ",
			method:          " get ",
			pathTemplate:    " /api/v1/users ",
			wantName:        "Read users",
			wantDescription: "List users",
			wantModule:      "user",
			wantIdentity: permissiondomain.RouteIdentity{
				Method:       "GET",
				PathTemplate: "/api/v1/users",
			},
		},
		{
			name:            "accepts api v1 root path",
			permissionName:  "Root",
			description:     "",
			module:          "system",
			method:          "POST",
			pathTemplate:    "/api/v1",
			wantName:        "Root",
			wantDescription: "",
			wantModule:      "system",
			wantIdentity: permissiondomain.RouteIdentity{
				Method:       "POST",
				PathTemplate: "/api/v1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotDescription, gotModule, gotIdentity, err := NormalizePermissionFields(
				tt.permissionName,
				tt.description,
				tt.module,
				tt.method,
				tt.pathTemplate,
			)

			require.NoError(t, err)
			require.Equal(t, tt.wantName, gotName)
			require.Equal(t, tt.wantDescription, gotDescription)
			require.Equal(t, tt.wantModule, gotModule)
			require.Equal(t, tt.wantIdentity, gotIdentity)
		})
	}
}

func TestNormalizePermissionFieldsRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name         string
		permission   string
		module       string
		method       string
		pathTemplate string
	}{
		{name: "blank permission name", permission: " ", module: "user", method: "GET", pathTemplate: "/api/v1/users"},
		{name: "blank module", permission: "Read users", module: " ", method: "GET", pathTemplate: "/api/v1/users"},
		{name: "unsupported method", permission: "Read users", module: "user", method: "OPTIONS", pathTemplate: "/api/v1/users"},
		{name: "relative path", permission: "Read users", module: "user", method: "GET", pathTemplate: "api/v1/users"},
		{name: "path outside api v1", permission: "Read users", module: "user", method: "GET", pathTemplate: "/internal/users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := NormalizePermissionFields(tt.permission, "description", tt.module, tt.method, tt.pathTemplate)

			require.ErrorIs(t, err, permissiondomain.ErrPermissionInvalid,
				"err = %v, want ErrPermissionInvalid", err)
		})
	}
}

func TestNormalizeOptionalHTTPMethod(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		want    string
		wantErr bool
	}{
		{name: "blank method", method: " ", want: ""},
		{name: "normalizes method", method: " patch ", want: "PATCH"},
		{name: "rejects unsupported method", method: "HEAD", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeOptionalHTTPMethod(tt.method)

			if tt.wantErr {
				require.ErrorIs(t, err, permissiondomain.ErrPermissionInvalid,
					"err = %v, want ErrPermissionInvalid", err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeOptionalModule(t *testing.T) {
	require.Equal(t, "user", NormalizeOptionalModule("  user  "))
	require.Empty(t, NormalizeOptionalModule("  "))
}
