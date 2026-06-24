//go:build apitest

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ---- panel ground-truth probing (shared across areas) ----------------------
//
// These read the panels DIRECTLY (via the xui client) to assert the real Xray state
// subgen produced. They are the whole point of apitest: the API says it did X; these
// confirm the panel actually reflects X.

// ClientUUID returns the uuid of client `email` on the inbound at `port` of the panel,
// or "" if absent (reads settings.clients — the authoritative list).
func (s *Base) ClientUUID(p Panel, port int, email string) string {
	inbs, err := s.xc.ListInbounds(Ctx, p.Target)
	s.Require().NoError(err)

	for _, in := range inbs {
		if in.Port != port {
			continue
		}

		for _, sc := range in.Clients {
			if sc.Email == email {
				return sc.UUID.String()
			}
		}
	}

	return ""
}

// PanelInboundID returns the numeric 3x-ui inbound id at `port` on the panel (needed
// to seed a client directly via the xui client, e.g. the email-collision orphan).
func (s *Base) PanelInboundID(p Panel, port int) int {
	inbs, err := s.xc.ListInbounds(Ctx, p.Target)
	s.Require().NoError(err)

	for _, in := range inbs {
		if in.Port == port {
			return in.ID
		}
	}

	s.Require().Failf("no inbound", "panel has no inbound on port %d", port)

	return 0
}

// RequireClient asserts client `email` is on (panel, port) and returns its uuid.
func (s *Base) RequireClient(p Panel, port int, email string) string {
	id := s.ClientUUID(p, port, email)
	s.Require().NotEmpty(id, "client %q must be present on port %d", email, port)

	return id
}

// RequireNoClient asserts client `email` is not on (panel, port).
func (s *Base) RequireNoClient(p Panel, port int, email string) {
	s.Require().Empty(s.ClientUUID(p, port, email), "client %q must be absent on port %d", email, port)
}

// ---- panel bootstrap over the 3x-ui HTTP API -------------------------------

// ensureInbound creates a minimal vless/tcp/none inbound on the port if absent.
func ensureInbound(p Panel, port int, remark string) error {
	if has, err := panelHasInbound(p, port); err != nil {
		return err
	} else if has {
		return nil
	}

	add := map[string]any{
		"up": 0, "down": 0, "total": 0, "remark": remark, "enable": true, "expiryTime": 0,
		"listen": "", "port": port, "protocol": "vless",
		"settings":       `{"clients":[],"decryption":"none","fallbacks":[]}`,
		"streamSettings": `{"network":"tcp","security":"none"}`,
		"sniffing":       `{"enabled":false,"destOverride":[]}`,
		"allocate":       `{"strategy":"always","refresh":5,"concurrency":3}`,
	}
	raw, _ := json.Marshal(add)

	resp, err := panelDo(http.MethodPost, p, "/panel/api/inbounds/add", raw)
	if err != nil {
		return err
	}

	var res struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}

	_ = json.Unmarshal(resp, &res)

	if !res.Success {
		// The panels are shared across the parallel suites, so another suite may have
		// added the same inbound between our check and our add — the UNIQUE tag then
		// rejects ours. Accept it as long as the inbound now exists.
		if has, herr := panelHasInbound(p, port); herr == nil && has {
			return nil
		}

		return fmt.Errorf("add inbound :%d: %s", port, res.Msg)
	}

	return nil
}

// panelHasInbound reports whether the panel already has an inbound on the given port.
func panelHasInbound(p Panel, port int) (bool, error) {
	body, err := panelDo(http.MethodGet, p, "/panel/api/inbounds/list", nil)
	if err != nil {
		return false, err
	}

	var lr struct {
		Obj []struct {
			Port int `json:"port"`
		} `json:"obj"`
	}

	_ = json.Unmarshal(body, &lr)

	for _, in := range lr.Obj {
		if in.Port == port {
			return true, nil
		}
	}

	return false, nil
}

func panelDo(method string, p Panel, path string, body []byte) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, p.URL+path, rd)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.Token)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
