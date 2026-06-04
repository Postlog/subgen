//go:build apitest

package api

import "fmt"

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
	ID       int64         `json:"id"`
	Name     string        `json:"name"`
	Sub      UserSub       `json:"sub"`
	Inbounds []UserInbound `json:"inbounds"`
	Stats    UserStats     `json:"stats"`
}

// CreateUser POSTs /admin/api/users/create (nickname + the inbound ids to bind).
func (c *Client) CreateUser(name string, inboundIDs []int64) (Result, error) {
	return c.post("/admin/api/users/create", map[string]any{"name": name, "inboundIDs": inboundIDs})
}

// EditUser POSTs /admin/api/users/edit (re-bind a user to a new inbound-id set).
func (c *Client) EditUser(id int64, inboundIDs []int64) (Result, error) {
	return c.post("/admin/api/users/edit", map[string]any{"id": id, "inboundIDs": inboundIDs})
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

// ListUsers GETs /admin/api/users and returns the rows.
func (c *Client) ListUsers() ([]User, error) {
	var resp struct {
		Users []User `json:"users"`
	}

	if err := c.getJSON("/admin/api/users", &resp); err != nil {
		return nil, err
	}

	return resp.Users, nil
}

// FindUser GETs the users list and returns the row whose nickname matches, or nil.
// user_create only returns {ok}, so this is how a scenario recovers the created
// user's id/subId/sub-URL to drive the rest of its lifecycle.
func (c *Client) FindUser(name string) (*User, error) {
	users, err := c.ListUsers()
	if err != nil {
		return nil, err
	}

	for i := range users {
		if users[i].Name == name {
			return &users[i], nil
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
