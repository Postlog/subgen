package xui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
)

var testClientID = uuid.MustParse("11111111-1111-4111-8111-111111111111")

func TestClient_AddClient(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "success", status: http.StatusOK, body: `{"success":true}`},
		{name: "error.envelope", status: http.StatusOK, body: `{"success":false,"msg":"dup"}`, wantErr: true},
		{name: "error.status", status: http.StatusBadRequest, body: ``, wantErr: true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotBody map[string]any

			c, tgt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/panel/api/clients/add", r.URL.Path)
				assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
				_ = json.NewDecoder(r.Body).Decode(&gotBody)

				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})

			err := c.AddClient(context.Background(), tgt, []int{1, 2},
				entity.ClientSpec{ID: testClientID, Email: "bob", SubID: "sub"})
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			cl, ok := gotBody["client"].(map[string]any)
			require.True(t, ok, "client object present")
			assert.Equal(t, testClientID.String(), cl["id"])
			assert.Equal(t, "bob", cl["email"])
			assert.Equal(t, "sub", cl["subId"])
			assert.Equal(t, float64(0), cl["tgId"], "tgId must be int 0, not string")
			assert.Equal(t, []any{float64(1), float64(2)}, gotBody["inboundIds"])
		})
	}
}
