package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

func TestWriteJSON(t *testing.T) {
	tt := []struct {
		name string

		ok  bool
		msg string

		wantBody string
	}{
		{name: "success.msg_field", ok: true, msg: "готово", wantBody: `{"msg":"готово","ok":true}`},
		{name: "error.err_field", ok: false, msg: "сломалось", wantBody: `{"err":"сломалось","ok":false}`},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rr := httptest.NewRecorder()

			WriteJSON(rr, tc.ok, tc.msg)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"))
			assert.JSONEq(t, tc.wantBody, rr.Body.String())
		})
	}
}

func TestJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	JSON(rr, map[string]any{"name": "postlog", "n": 3})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"name":"postlog","n":3}`, rr.Body.String())
}

func TestJSONResult(t *testing.T) {
	tt := []struct {
		name string

		okMsg string
		err   error

		wantBody string
	}{
		{name: "success.no_error", okMsg: "сохранено", wantBody: `{"msg":"сохранено","ok":true}`},
		{
			name:     "error.maps_sentinel",
			okMsg:    "сохранено",
			err:      entity.ErrNameTaken,
			wantBody: `{"err":"Имя занято","ok":false}`,
		},
		{
			name:     "error.passthrough_unknown",
			okMsg:    "сохранено",
			err:      errors.New("boom"),
			wantBody: `{"err":"boom","ok":false}`,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rr := httptest.NewRecorder()

			JSONResult(rr, tc.okMsg, tc.err)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.JSONEq(t, tc.wantBody, rr.Body.String())
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	type dst struct {
		Name string  `json:"name"`
		IDs  []int64 `json:"ids"`
	}

	tt := []struct {
		name string

		body string

		want    dst
		wantErr bool
	}{
		{
			name: "success.decodes_fields",
			body: `{"name":"postlog","ids":[1,2,3]}`,
			want: dst{Name: "postlog", IDs: []int64{1, 2, 3}},
		},
		{name: "error.malformed", body: `{bad`, wantErr: true},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))

			var got dst

			err := DecodeJSON(r, &got)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestUserMessage(t *testing.T) {
	tt := []struct {
		name string

		err  error
		want string
	}{
		{
			name: "panel_client_exists",
			err:  entity.PanelClientExistsError{Node: "🇷🇺 RU1"},
			want: "на панели «🇷🇺 RU1» уже есть клиент с таким именем — удалите его там вручную или выберите другое имя",
		},
		{name: "invalid_user_name", err: entity.ErrInvalidUserName, want: msgInvalidUserName},
		{name: "name_taken", err: entity.ErrNameTaken, want: msgNameTaken},
		{name: "node_name_taken", err: entity.ErrNodeNameTaken, want: msgNodeNameTaken},
		{name: "inbound_duplicate", err: entity.ErrInboundDuplicate, want: msgInboundDuplicate},
		{name: "rule_provider_name_taken", err: entity.ErrRuleProviderNameTaken, want: msgProviderNameTaken},
		{name: "no_connection_selected", err: entity.ErrNoConnectionSelected, want: msgNoConnection},
		{name: "node_not_found", err: entity.ErrNodeNotFound, want: msgNodeNotFound},
		{name: "inbound_not_found", err: entity.ErrInboundNotFound, want: msgInboundNotFound},
		{name: "group_name_empty", err: mihomo.ErrGroupNameEmpty, want: msgGroupNameEmpty},
		{name: "group_name_taken", err: mihomo.ErrGroupNameTaken, want: msgGroupNameTaken},
		{name: "group_unknown_type", err: mihomo.ErrGroupUnknownType, want: msgGroupUnknownType},
		{name: "group_no_members", err: mihomo.ErrGroupNoMembers, want: msgGroupNoMembers},
		{name: "group_cycle", err: mihomo.ErrGroupCycle, want: msgGroupCycle},
		{name: "bad_ref", err: mihomo.ErrBadRef, want: msgBadRef},
		{name: "group_ref_range", err: mihomo.ErrGroupRefRange, want: msgGroupRefRange},
		{name: "unknown_rule_type", err: mihomo.ErrUnknownRuleType, want: msgUnknownRuleType},
		{name: "match_not_last", err: mihomo.ErrMatchNotLast, want: msgMatchNotLast},
		{name: "rule_value_required", err: mihomo.ErrRuleValueRequired, want: msgRuleValueReq},
		{name: "base_yaml_invalid", err: mihomo.ErrBaseYAMLInvalid, want: msgBaseYAMLInvalid},
		{name: "generated_key_present", err: mihomo.ErrGeneratedKeyPresent, want: msgGeneratedKey},
		{name: "provider_name_empty", err: mihomo.ErrProviderNameEmpty, want: msgProviderNameEmpty},
		{name: "provider_bad_behavior", err: mihomo.ErrProviderBadBehavior, want: msgProviderBadBehavior},
		{name: "provider_bad_format", err: mihomo.ErrProviderBadFormat, want: msgProviderBadFormat},
		{name: "provider_url_empty", err: mihomo.ErrProviderURLEmpty, want: msgProviderURLEmpty},
		{name: "rule_set_unknown_provider", err: mihomo.ErrRuleSetUnknownProvider, want: msgRuleSetUnknownProv},
		{name: "wrapped_sentinel.unwraps", err: fmt.Errorf("ctx: %w", entity.ErrNameTaken), want: msgNameTaken},
		{name: "unknown.passthrough", err: errors.New("raw operational detail"), want: "raw operational detail"},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, UserMessage(tc.err))
		})
	}
}
