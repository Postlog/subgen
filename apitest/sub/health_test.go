//go:build apitest

package sub_test

import "net/http"

// Corner cases considered for GET /healthz:
//   - ok — returns 200 with a plain-text "ok" body (the liveness contract the boot
//          poller and any orchestrator rely on).

// TestHealthz covers the liveness probe.
func (s *SubSuite) TestHealthz() {
	resp, err := s.api.HealthzRaw()
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.Status)
	s.Contains(string(resp.Body), "ok")
	s.Contains(resp.Headers.Get("Content-Type"), "text/plain")
}
