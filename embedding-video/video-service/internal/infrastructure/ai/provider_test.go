package ai

import "testing"

func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ProviderLegacy},
		{name: "legacy", in: "legacy", want: ProviderLegacy},
		{name: "eino", in: "eino", want: ProviderEino},
		{name: "case and spaces", in: " EINO ", want: ProviderEino},
		{name: "unknown", in: "experimental", want: ProviderLegacy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeProvider(tt.in); got != tt.want {
				t.Fatalf("NormalizeProvider(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
