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
		name      string
		deleteErr error // what the nodes service Delete returns
		result    oas.NodeDeleteRes
		err       error
	}{
		{name: "success", result: &oas.MessageResponse{Message: MsgDeleted}},
		{name: "error.not_found", deleteErr: entity.ErrNodeNotFound, result: &oas.NodeDeleteBadRequest{ErrMessage: MsgNotFound}},
		{name: "error.referenced", deleteErr: entity.ErrInboundReferenced, result: &oas.NodeDeleteBadRequest{ErrMessage: MsgInboundReferenced}},
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
			assert.Equal(t, tc.result, res)
		})
	}
}
