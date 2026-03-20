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
	require.Contains(s.T(), string(body), "AutoSkills Dashboard")
	require.Contains(s.T(), string(body), "Set up your workspace")
	require.Contains(s.T(), string(body), "Why your skill set changed")
	require.Contains(s.T(), string(body), "Version history")
}
