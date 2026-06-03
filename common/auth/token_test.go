package auth

import "testing"

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
			if got := StripBearerPrefix(tt.input); got != tt.want {
				t.Fatalf("StripBearerPrefix(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}
