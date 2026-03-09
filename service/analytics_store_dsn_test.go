package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultAnalyticsStoreDSNUsesSQLiteFallback(t *testing.T) {
	require.Equal(t, "file:agentopt-store?mode=memory&cache=shared&_fk=1", defaultAnalyticsStoreDSN(""))
	require.Equal(t, "data/agentopt-store.db", defaultAnalyticsStoreDSN("data/agentopt-store.json"))
	require.Equal(t, "data/agentopt-store.db", defaultAnalyticsStoreDSN("data/agentopt-store"))
	require.Equal(t, "data/agentopt-local.db?_fk=1", defaultAnalyticsStoreDSN("data/agentopt-local.db?_fk=1"))
}
