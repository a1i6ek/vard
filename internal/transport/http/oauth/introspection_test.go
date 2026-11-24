package oauth

import "testing"

func TestTheSignedIntrospectionIsAskedForByMediaType(t *testing.T) {
	tests := []struct {
		accept string
		want   bool
	}{
		{accept: "application/token-introspection+jwt", want: true},
		{accept: "application/json, application/token-introspection+jwt;q=0.9", want: true},
		{accept: "Application/Token-Introspection+JWT", want: true},
		{accept: "application/json", want: false},
		{accept: "", want: false},
		{accept: "*/*", want: false},
	}
	for _, tt := range tests {
		if got := acceptsIntrospectionJWT(tt.accept); got != tt.want {
			t.Errorf("Accept %q: signed = %v, want %v", tt.accept, got, tt.want)
		}
	}
}
