package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBearerAuthorization(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "bearer token", input: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "lowercase bearer token", input: "bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "uppercase bearer token", input: "BEARER abc.def.ghi", want: "abc.def.ghi"},
		{name: "trim bearer token", input: "  Bearer abc.def.ghi  ", want: "abc.def.ghi"},
		{name: "trim token after prefix", input: "Bearer   abc.def.ghi  ", want: "abc.def.ghi"},
		{name: "preserve token case", input: "Bearer AbC.Def.GhI", want: "AbC.Def.GhI"},
		{name: "missing prefix", input: "abc.def.ghi", wantErr: ErrMissingBearerPrefix},
		{name: "invalid prefix", input: "Token abc.def.ghi", wantErr: ErrMissingBearerPrefix},
		{name: "empty token", input: "Bearer ", wantErr: ErrEmptyBearerToken},
		{name: "blank header", input: "   ", wantErr: ErrMissingBearerPrefix},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBearerAuthorization(tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestStripBearerPrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "raw token", input: "abc.def.ghi", want: "abc.def.ghi"},
		{name: "trim raw token", input: "  abc.def.ghi  ", want: "abc.def.ghi"},
		{name: "bearer token", input: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "mixed case bearer token", input: "bEaReR abc.def.ghi", want: "abc.def.ghi"},
		{name: "trim bearer token", input: "  Bearer abc.def.ghi  ", want: "abc.def.ghi"},
		{name: "trim token after prefix", input: "Bearer   abc.def.ghi  ", want: "abc.def.ghi"},
		{name: "empty token", input: "   ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, StripBearerPrefix(tt.input))
		})
	}
}
