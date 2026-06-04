package token

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMake(t *testing.T) {
	t.Parallel()

	got := Make("secret", "sub-123")

	assert.Len(t, got, length)
	assert.Equal(t, got, Make("secret", "sub-123"), "deterministic for the same secret+subID")
	assert.NotEqual(t, got, Make("other", "sub-123"), "rotating the secret changes the token")
	assert.NotEqual(t, got, Make("secret", "sub-456"), "a different subID changes the token")
}

func TestMatch(t *testing.T) {
	t.Parallel()

	const secret, subID = "secret", "sub-123"

	tt := []struct {
		name  string
		token string

		want bool
	}{
		{name: "accepts.correct", token: Make(secret, subID), want: true},
		{name: "rejects.wrong", token: "deadbeefdeadbeefdeadbeef", want: false},
		{name: "rejects.other_secret", token: Make("other", subID), want: false},
		{name: "rejects.empty", token: "", want: false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := Match(secret, subID, tc.token)

			assert.Equal(t, tc.want, got)
		})
	}
}
