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

// wantNode is the entity.Node the handler builds from nodeReq() (trimmed, ID/Token absent,
// blank rows dropped) — the exact value the nodes service Save must receive.
func wantNode() entity.Node {
	return entity.Node{
		Name: "RU1", VPNHost: "host.example", PanelBaseURL: "https://panel.example:8443", PanelBasePath: "/",
		Inbounds: []entity.Inbound{{Name: "smart", Port: 8443}},
	}
}

func TestHandler_NodeSave(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name    string
		saveErr error // what the nodes service Save returns
		result  oas.NodeSaveRes
		err     error
	}{
		{name: "success", result: &oas.MessageResponse{Message: "Узел сохранён: RU1"}},
		{name: "error.name_taken", saveErr: entity.ErrNodeNameTaken, result: &oas.NodeSaveConflict{ErrMessage: MsgNodeNameTaken}},
		{name: "error.inbound_duplicate", saveErr: entity.ErrInboundDuplicate, result: &oas.NodeSaveConflict{ErrMessage: MsgInboundDuplicate}},
		{name: "error.inbound_referenced", saveErr: entity.ErrInboundReferenced, result: &oas.NodeSaveBadRequest{ErrMessage: MsgInboundReferenced}},
		{name: "error.invalid_node_name", saveErr: entity.ErrValidationNodeName, result: &oas.NodeSaveBadRequest{ErrMessage: MsgNodeName}},
		{name: "error.invalid_host", saveErr: entity.ErrValidationHost, result: &oas.NodeSaveBadRequest{ErrMessage: MsgHost}},
		{name: "error.invalid_panel_url", saveErr: entity.ErrValidationPanelURL, result: &oas.NodeSaveBadRequest{ErrMessage: MsgPanelURL}},
		{name: "error.no_path", saveErr: entity.ErrValidationBasePath, result: &oas.NodeSaveBadRequest{ErrMessage: MsgBasePath}},
		{name: "error.no_inbounds", saveErr: entity.ErrValidationNoInbounds, result: &oas.NodeSaveBadRequest{ErrMessage: MsgNoInbounds}},
		{name: "error.bad_inbound_name", saveErr: entity.ErrValidationInboundName, result: &oas.NodeSaveBadRequest{ErrMessage: MsgInboundName}},
		{name: "error.bad_inbound_port", saveErr: entity.ErrValidationInboundPort, result: &oas.NodeSaveBadRequest{ErrMessage: MsgInboundPort}},
		{name: "error.dup_inbound_name", saveErr: entity.ErrValidationInboundNameUq, result: &oas.NodeSaveBadRequest{ErrMessage: MsgInboundNameUq}},
		{name: "error.dup_inbound_port", saveErr: entity.ErrValidationInboundPortUq, result: &oas.NodeSaveBadRequest{ErrMessage: MsgInboundPortUq}},
		{name: "error.internal", saveErr: internalErr, err: internalErr},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			svc := NewMocknodesService(ctrl)
			svc.EXPECT().Save(gomock.Any(), wantNode()).Return(int64(1), tc.saveErr)

			res, err := New(svc).NodeSave(context.Background(), nodeReq())

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
