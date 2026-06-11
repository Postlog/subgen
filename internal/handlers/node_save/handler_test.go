package node_save

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

func nodeReq() *oas.NodeSaveReq {
	return &oas.NodeSaveReq{
		Name: "RU1", VpnHost: "host.example", PanelBaseURL: "https://panel.example:8443", PanelBasePath: "/",
		Inbounds: []oas.NodeSaveReqInboundsItem{{Name: "smart", Port: 8443}},
	}
}

func TestHandler_NodeSave(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name        string
		saveErr     error // what the nodes service Save returns
		result      oas.NodeSaveRes
		wantBlocked bool // assert a NodeSaveBadRequest with a non-empty (formatted) message
		err         error
	}{
		{name: "success", result: &oas.MessageResponse{Message: "Узел сохранён: RU1"}},
		{name: "error.name_taken", saveErr: entity.ErrNodeNameTaken, result: &oas.NodeSaveConflict{ErrMessage: msgNodeNameTaken}},
		{name: "error.inbound_duplicate", saveErr: entity.ErrInboundDuplicate, result: &oas.NodeSaveConflict{ErrMessage: msgInboundDuplicate}},
		{name: "error.invalid_node_name", saveErr: entity.ErrValidationNodeName, result: &oas.NodeSaveBadRequest{ErrMessage: msgNodeName}},
		{name: "error.invalid_host", saveErr: entity.ErrValidationHost, result: &oas.NodeSaveBadRequest{ErrMessage: msgHost}},
		{name: "error.invalid_panel_url", saveErr: entity.ErrValidationPanelURL, result: &oas.NodeSaveBadRequest{ErrMessage: msgPanelURL}},
		{name: "error.no_path", saveErr: entity.ErrValidationBasePath, result: &oas.NodeSaveBadRequest{ErrMessage: msgBasePath}},
		{name: "error.no_inbounds", saveErr: entity.ErrValidationNoInbounds, result: &oas.NodeSaveBadRequest{ErrMessage: msgNoInbounds}},
		{name: "error.bad_inbound_name", saveErr: entity.ErrValidationInboundName, result: &oas.NodeSaveBadRequest{ErrMessage: msgInboundName}},
		{name: "error.bad_inbound_port", saveErr: entity.ErrValidationInboundPort, result: &oas.NodeSaveBadRequest{ErrMessage: msgInboundPort}},
		{name: "error.dup_inbound_name", saveErr: entity.ErrValidationInboundNameUq, result: &oas.NodeSaveBadRequest{ErrMessage: msgInboundNameUq}},
		{name: "error.dup_inbound_port", saveErr: entity.ErrValidationInboundPortUq, result: &oas.NodeSaveBadRequest{ErrMessage: msgInboundPortUq}},
		{
			name:        "error.inbounds_blocked",
			saveErr:     entity.InboundsBlockedError{Inbounds: []entity.BlockedInbound{{Label: "RU1-force:8443", Users: 2}}},
			wantBlocked: true,
		},
		{name: "error.internal", saveErr: internalErr, err: internalErr},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			svc := NewMocknodeSaver(ctrl)
			svc.EXPECT().Save(gomock.Any(), gomock.Any()).Return(int64(1), tc.saveErr)

			res, err := New(svc).NodeSave(context.Background(), nodeReq())

			require.ErrorIs(t, err, tc.err)

			if tc.wantBlocked {
				bad, ok := res.(*oas.NodeSaveBadRequest)
				require.True(t, ok, "want *oas.NodeSaveBadRequest, got %T", res)
				assert.NotEmpty(t, bad.ErrMessage)

				return
			}

			assert.Equal(t, tc.result, res)
		})
	}
}
