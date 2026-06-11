package node_delete

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

func TestHandler_NodeDelete(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name        string
		deleteErr   error // what the nodes service Delete returns
		result      oas.NodeDeleteRes
		wantBlocked bool
		err         error
	}{
		{name: "success", result: &oas.MessageResponse{Message: msgDeleted}},
		{
			name:        "error.blocked",
			deleteErr:   entity.InboundsBlockedError{Inbounds: []entity.BlockedInbound{{Label: "RU1-force:8443", Users: 2}}},
			wantBlocked: true,
		},
		{name: "error.internal", deleteErr: internalErr, err: internalErr},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			svc := NewMocknodeDeleter(ctrl)
			svc.EXPECT().Delete(gomock.Any(), int64(7)).Return(tc.deleteErr)

			res, err := New(svc).NodeDelete(context.Background(), &oas.NodeDeleteReq{ID: 7})

			require.ErrorIs(t, err, tc.err)

			if tc.wantBlocked {
				bad, ok := res.(*oas.NodeDeleteBadRequest)
				require.True(t, ok, "want *oas.NodeDeleteBadRequest, got %T", res)
				assert.NotEmpty(t, bad.ErrMessage)

				return
			}

			assert.Equal(t, tc.result, res)
		})
	}
}
