package xui

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
)

func TestClient_ListInbounds(t *testing.T) {
	t.Parallel()

	clientUUID := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	// One inbound; settings + streamSettings arrive as JSON-encoded *strings*
	// (3x-ui quirk) so this also exercises decode()/unwrapJSON.
	const okBody = `{"success":true,"msg":"","obj":[{"id":1,"port":443,"remark":"in-443","enable":true,"settings":"{\"clients\":[{\"id\":\"11111111-1111-4111-8111-111111111111\",\"email\":\"alice\",\"flow\":\"vision\",\"subId\":\"sub1\",\"enable\":true}]}","streamSettings":"{\"network\":\"tcp\",\"security\":\"reality\",\"realitySettings\":{\"serverNames\":[\"example.com\"],\"shortIds\":[\"ab\"],\"settings\":{\"publicKey\":\"PK\",\"fingerprint\":\"chrome\"}}}","clientStats":[{"email":"alice","uuid":"11111111-1111-4111-8111-111111111111","subId":"sub1","up":10,"down":20,"total":0,"expiryTime":0,"enable":true}]}]}`

	tt := []struct {
		name   string
		status int
		body   string

		result  []entity.PanelInbound
		wantErr bool
	}{
		{
			name: "success", status: http.StatusOK, body: okBody,
			result: []entity.PanelInbound{{
				ID:     1,
				Port:   443,
				Remark: "in-443",
				Enable: true,
				Stream: entity.StreamInfo{
					Network:     "tcp",
					Security:    "reality",
					PublicKey:   "PK",
					ShortID:     "ab",
					ServerName:  "example.com",
					Fingerprint: "chrome",
				},
				Clients: []entity.PanelClient{
					{UUID: clientUUID, Email: "alice", Flow: "vision", SubID: "sub1"},
				},
				Stats: []entity.PanelClientStat{
					{Email: "alice", UUID: clientUUID, SubID: "sub1", Up: 10, Down: 20, Enable: true},
				},
			}},
		},
		{name: "empty", status: http.StatusOK, body: `{"success":true,"obj":[]}`, result: []entity.PanelInbound{}},
		{name: "error.envelope", status: http.StatusOK, body: `{"success":false,"msg":"boom"}`, wantErr: true},
		{name: "error.parse", status: http.StatusOK, body: `not json`, wantErr: true},
		{name: "error.status", status: http.StatusInternalServerError, body: `{}`, wantErr: true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, tgt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/panel/api/inbounds/list", r.URL.Path)
				assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})

			out, err := c.ListInbounds(context.Background(), tgt)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.result, out)
		})
	}
}
