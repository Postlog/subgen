//go:build apitest

package api

import (
	"fmt"
	"net/url"
)

// The User* read DTOs below are hand-rolled to mirror the users-list response
// field-for-field — independent of the server's generated types, so the wire shape is
// actually asserted. Request bodies are likewise plain maps (see the methods below), not
// generated request types, so the request mapping is exercised too.

// UserInbound is one (user, inbound) binding as the users API reports it.
type UserInbound struct {
	ID      int64  `json:"id"`
	Label   string `json:"label"`
	Port    int    `json:"port"`
	Missing bool   `json:"missing"`
}

// UserSub is a user's subscription coordinates: the shared subId and the absolute
// /sub URL (token-signed) the user fetches.
type UserSub struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// UserStats is a user's aggregated traffic, as the users API computes from the fleet
// snapshot.
type UserStats struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// User is one row of the users API (GET /admin/api/users).
type User struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Sub         UserSub       `json:"sub"`
	Inbounds    []UserInbound `json:"inbounds"`
	Stats       UserStats     `json:"stats"`
}

// CreateUser POSTs /admin/api/users/create (nickname + the inbound ids to bind).
func (c *Client) CreateUser(name string, inboundIDs []int64) (Result, error) {
	return c.post("/admin/api/users/create", map[string]any{"name": name, "inboundIDs": inboundIDs})
}

// CreateUserWith POSTs /admin/api/users/create with an optional free-text description
// (the description-less CreateUser stays churn-free for the many scenarios that ignore it).
func (c *Client) CreateUserWith(name string, inboundIDs []int64, description string) (Result, error) {
	return c.post("/admin/api/users/create", map[string]any{"name": name, "inboundIDs": inboundIDs, "description": description})
}

// EditUser POSTs /admin/api/users/edit (re-bind a user to a new inbound-id set).
func (c *Client) EditUser(id int64, inboundIDs []int64) (Result, error) {
	return c.post("/admin/api/users/edit", map[string]any{"id": id, "inboundIDs": inboundIDs})
}

// EditUserWith POSTs /admin/api/users/edit with a description alongside the new inbound set.
func (c *Client) EditUserWith(id int64, inboundIDs []int64, description string) (Result, error) {
	return c.post("/admin/api/users/edit", map[string]any{"id": id, "inboundIDs": inboundIDs, "description": description})
}

// DeleteUser POSTs /admin/api/users/delete.
func (c *Client) DeleteUser(id int64) (Result, error) {
	return c.post("/admin/api/users/delete", map[string]any{"id": id})
}

// RecreateUser POSTs /admin/api/users/recreate (re-provision panel clients from the
// store after drift).
func (c *Client) RecreateUser(id int64) (Result, error) {
	return c.post("/admin/api/users/recreate", map[string]any{"id": id})
}

// ListUsers GETs /admin/api/users and returns the rows. The list is server-paged; a
// large perPage keeps the small test fleet on one page.
func (c *Client) ListUsers() ([]User, error) {
	var resp struct {
		Users []User `json:"users"`
	}

	if err := c.getJSON("/admin/api/users?perPage=200", &resp); err != nil {
		return nil, err
	}

	return resp.Users, nil
}

// FindUser GETs the users list filtered by the nickname (server-side ?q= search) and
// returns the matching row, or nil. user_create only returns a message (not the new
// row), so this is how a scenario recovers the created user's id/subId/sub-URL to drive
// the rest of its lifecycle — going through the filter so paging never hides a
// freshly-created user.
func (c *Client) FindUser(name string) (*User, error) {
	var resp struct {
		Users []User `json:"users"`
	}

	if err := c.getJSON("/admin/api/users?perPage=200&q="+url.QueryEscape(name), &resp); err != nil {
		return nil, err
	}

	for i := range resp.Users {
		if resp.Users[i].Name == name {
			return &resp.Users[i], nil
		}
	}

	return nil, nil
}

// MustFindUser is FindUser that errors if the user is absent (the common case after a
// successful create).
func (c *Client) MustFindUser(name string) (*User, error) {
	u, err := c.FindUser(name)
	if err != nil {
		return nil, err
	}

	if u == nil {
		return nil, fmt.Errorf("user %q not found in users list", name)
	}

	return u, nil
}
