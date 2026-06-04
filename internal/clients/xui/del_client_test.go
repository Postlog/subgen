package xui

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_DelClient(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "success", status: http.StatusOK, body: `{"success":true}`},
		{name: "error.envelope", status: http.StatusOK, body: `{"success":false,"msg":"not found"}`, wantErr: true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string

			c, tgt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})

			err := c.DelClient(context.Background(), tgt, "bob")
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "/panel/api/clients/del/bob", gotPath)
		})
	}
}
