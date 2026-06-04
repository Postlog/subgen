package node_delete

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
)

// node7 is a one-inbound node: "main" :4433 (inbound id 10).
func node7() *entity.Node {
	return &entity.Node{
		ID: 7, Name: "N1",
		Inbounds: []entity.Inbound{{ID: 10, Name: "main", Port: 4433}},
	}
}

type mocks struct {
	nodes   *MocknodeRepo
	routing *MockroutingRepo
	cache   *MockcacheInvalidator
}

func TestHandler_ServeHTTP(t *testing.T) {
	targetErr := errors.New("test")

	tt := []struct {
		name       string
		id         string
		buildMocks func(m *mocks)
		wantOK     bool
	}{
		{
			name: "error.users_blocking", id: "7",
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(node7(), nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{10}).Return(map[int64]int{10: 2}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
			},
		},
		{
			name: "error.rule_blocking", id: "7",
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(node7(), nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{10}).Return(map[int64]int{10: 1}, nil)
			},
		},
		{
			name: "error.delete", id: "7",
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(node7(), nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.nodes.EXPECT().Delete(gomock.Any(), int64(7)).Return(targetErr)
			},
		},
		{
			name: "success", id: "7", wantOK: true,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(node7(), nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.nodes.EXPECT().Delete(gomock.Any(), int64(7)).Return(nil)
				m.cache.EXPECT().Invalidate()
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := &mocks{nodes: NewMocknodeRepo(ctrl), routing: NewMockroutingRepo(ctrl), cache: NewMockcacheInvalidator(ctrl)}
			if tc.buildMocks != nil {
				tc.buildMocks(m)
			}

			h := New(m.nodes, m.routing, m.cache)
			req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes/delete",
				strings.NewReader(`{"id":`+tc.id+`}`))

			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			var body struct {
				OK bool `json:"ok"`
			}

			require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
			assert.Equal(t, tc.wantOK, body.OK)
		})
	}
}
