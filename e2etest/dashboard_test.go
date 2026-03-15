package e2etest

import (
	"io"
	"net/http"

	"github.com/stretchr/testify/require"
)

func (s *APISuite) TestDashboardPage_OK() {
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, s.c.BaseURL+"/dashboard", nil)
	require.NoError(s.T(), err)

	resp, err := s.c.HTTP.Do(req)
	require.NoError(s.T(), err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
	require.Contains(s.T(), string(body), "Analysis reports")
	require.Contains(s.T(), string(body), "Latest trace analysis")
	require.Contains(s.T(), string(body), "Collected session traces")
	require.Contains(s.T(), string(body), "Usage Analytics")
	require.Contains(s.T(), string(body), "What happened in your Codex sessions")
}
