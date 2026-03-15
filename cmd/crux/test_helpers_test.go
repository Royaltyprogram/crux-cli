package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/dto/request"
	"github.com/Royaltyprogram/aiops/dto/response"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	fn()

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	return strings.TrimSpace(string(output))
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stderr = writer
	defer func() {
		os.Stderr = original
	}()

	fn()

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	return strings.TrimSpace(string(output))
}

func mustJSONRawMessage(t *testing.T, value any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)
	return json.RawMessage(data)
}

func uploadedSessionBatchItem(req request.SessionSummaryReq) response.SessionBatchIngestItemResp {
	recordedAt := req.Timestamp
	return response.SessionBatchIngestItemResp{
		SessionID:  req.SessionID,
		ProjectID:  req.ProjectID,
		Status:     "uploaded",
		RecordedAt: &recordedAt,
	}
}

func sessionBatchResp(projectID string, items ...response.SessionBatchIngestItemResp) response.SessionBatchIngestResp {
	resp := response.SessionBatchIngestResp{
		SchemaVersion: reportAPISchemaVersion,
		ProjectID:     projectID,
		Accepted:      len(items),
		Items:         append([]response.SessionBatchIngestItemResp(nil), items...),
	}
	for _, item := range items {
		switch strings.TrimSpace(item.Status) {
		case "failed":
			resp.Failed++
		case "updated":
			resp.Updated++
		default:
			resp.Uploaded++
		}
	}
	if len(items) > 0 {
		now := time.Now().UTC()
		resp.ResearchStatus = &response.ReportResearchStatusResp{
			SchemaVersion: reportAPISchemaVersion,
			State:         "waiting_for_next_batch",
			Summary:       "Collected sessions in batch.",
			TriggeredAt:   &now,
		}
	}
	return resp
}
