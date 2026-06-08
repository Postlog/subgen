package users_api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/token"
)

const (
	testSecret = "s3cr3t"
	testBase   = "https://sub.example.com/"
)

// body mirrors the JSON the handler emits (page of rows + paging meta).
type body struct {
	Users   []row `json:"users"`
	Total   int64 `json:"total"`
	Page    int   `json:"page"`
	PerPage int   `json:"perPage"`
}

func TestHandler_ServeHTTP(t *testing.T) {
	downstreamErr := errors.New("store down")

	tt := []struct {
		name       string
		url        string
		buildMocks func(u *MockuserLister, f *MockfleetReader, got *entity.UserListParams)
		wantParams *entity.UserListParams
		wantStatus int
		wantBody   *body
	}{
		{
			name: "empty",
			url:  "/admin/api/users",
			buildMocks: func(u *MockuserLister, f *MockfleetReader, got *entity.UserListParams) {
				u.EXPECT().ListPage(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, p entity.UserListParams) (entity.UserPage, error) {
						*got = p

						return entity.UserPage{}, nil
					})
				f.EXPECT().Fleet(gomock.Any()).Return(emptyFleet(), nil)
			},
			wantParams: &entity.UserListParams{Limit: 50, Offset: 0},
			wantBody:   &body{Users: []row{}, Total: 0, Page: 1, PerPage: 50},
		},
		{
			name: "success.maps_rows_badges_stats",
			url:  "/admin/api/users",
			buildMocks: func(u *MockuserLister, f *MockfleetReader, got *entity.UserListParams) {
				u.EXPECT().ListPage(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, p entity.UserListParams) (entity.UserPage, error) {
						*got = p

						return entity.UserPage{
							Total: 1,
							Users: []entity.User{{
								ID: 7, Name: "amy", SubID: "sub-amy",
								Connections: []entity.Connection{
									{InboundID: 1, Node: "RU1", Name: "a", Port: 443}, // present
									{InboundID: 2, Node: "RU1", Name: "b", Port: 444}, // observed, absent → missing
									{InboundID: 3, Node: "RU2", Name: "c", Port: 445}, // node unobserved → not missing
								},
							}},
						}, nil
					})
				f.EXPECT().Fleet(gomock.Any()).Return(&entity.Fleet{
					Subs: map[string]*entity.Subscriber{"sub-amy": {SubID: "sub-amy", Up: 10, Down: 20}},
					ClientsByInbound: map[int64]map[string]bool{
						1: {"amy": true},
						2: {},
					},
				}, nil)
			},
			wantBody: &body{
				Total: 1, Page: 1, PerPage: 50,
				Users: []row{{
					ID: 7, Name: "amy",
					Sub: subInfo{ID: "sub-amy", URL: "https://sub.example.com/sub/mihomo/" + token.Make(testSecret, "sub-amy")},
					Inbounds: []inboundView{
						{ID: 1, Label: "RU1-a", Port: 443, Missing: false},
						{ID: 2, Label: "RU1-b", Port: 444, Missing: true},
						{ID: 3, Label: "RU2-c", Port: 445, Missing: false},
					},
					Stats: stats{Up: 10, Down: 20},
				}},
			},
		},
		{
			name: "query.parses_filters_and_paging",
			url:  "/admin/api/users?q=Foo&inbound=1&inbound=2&inbound=bad&page=3&perPage=10",
			buildMocks: func(u *MockuserLister, f *MockfleetReader, got *entity.UserListParams) {
				u.EXPECT().ListPage(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, p entity.UserListParams) (entity.UserPage, error) {
						*got = p

						return entity.UserPage{Total: 99}, nil
					})
				f.EXPECT().Fleet(gomock.Any()).Return(emptyFleet(), nil)
			},
			wantParams: &entity.UserListParams{NameQuery: "Foo", InboundIDs: []int64{1, 2}, Limit: 10, Offset: 20},
			wantBody:   &body{Users: []row{}, Total: 99, Page: 3, PerPage: 10},
		},
		{
			name: "query.perpage_clamped_and_page_floored",
			url:  "/admin/api/users?page=0&perPage=9999",
			buildMocks: func(u *MockuserLister, f *MockfleetReader, got *entity.UserListParams) {
				u.EXPECT().ListPage(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, p entity.UserListParams) (entity.UserPage, error) {
						*got = p

						return entity.UserPage{}, nil
					})
				f.EXPECT().Fleet(gomock.Any()).Return(emptyFleet(), nil)
			},
			wantParams: &entity.UserListParams{Limit: 200, Offset: 0},
			wantBody:   &body{Users: []row{}, Total: 0, Page: 1, PerPage: 200},
		},
		{
			name: "error.downstream",
			url:  "/admin/api/users",
			buildMocks: func(u *MockuserLister, f *MockfleetReader, got *entity.UserListParams) {
				u.EXPECT().ListPage(gomock.Any(), gomock.Any()).Return(entity.UserPage{}, downstreamErr)
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			users := NewMockuserLister(ctrl)
			fleetR := NewMockfleetReader(ctrl)

			var got entity.UserListParams
			if tc.buildMocks != nil {
				tc.buildMocks(users, fleetR, &got)
			}

			h := New(users, fleetR, testSecret, testBase)
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			status := tc.wantStatus
			if status == 0 {
				status = http.StatusOK
			}

			require.Equal(t, status, rec.Code)

			if tc.wantParams != nil {
				assert.Equal(t, *tc.wantParams, got)
			}

			if tc.wantBody != nil {
				var b body
				require.NoError(t, json.NewDecoder(rec.Body).Decode(&b))
				assert.Equal(t, *tc.wantBody, b)
			}
		})
	}
}

func emptyFleet() *entity.Fleet {
	return &entity.Fleet{Subs: map[string]*entity.Subscriber{}, ClientsByInbound: map[int64]map[string]bool{}}
}
