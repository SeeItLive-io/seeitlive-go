package seeitlive

import (
	"context"
	"net/url"
)

// Snapshots groups the snapshot-related API calls. Both require the team's
// plan to include snapshot history.
type Snapshots struct {
	http *httpClient
}

// List returns a session's snapshots, newest first.
// GET /sessions/{id}/snapshots.
func (s *Snapshots) List(ctx context.Context, sessionID string) ([]SnapshotSummary, error) {
	var out []SnapshotSummary
	path := "/sessions/" + url.PathEscape(sessionID) + "/snapshots"
	if err := s.http.request(ctx, path, requestOptions{}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get fetches one snapshot as base64 JSON. The endpoint serves raw PNG bytes
// by default; the SDK asks for ?format=base64 so the response stays JSON — and
// signs the query along with the path, which is what the server verifies.
// GET /snapshots/{id}?format=base64.
func (s *Snapshots) Get(ctx context.Context, id string) (*SnapshotImage, error) {
	var out SnapshotImage
	path := "/snapshots/" + url.PathEscape(id) + "?format=base64"
	if err := s.http.request(ctx, path, requestOptions{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
