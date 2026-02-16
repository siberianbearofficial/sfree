package manager

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/example/s3aas/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type stubSourceClient struct {
	uploaded [][]byte
}

func (c *stubSourceClient) Upload(_ context.Context, _ string, r io.Reader) (string, error) {
	data, _ := io.ReadAll(r)
	c.uploaded = append(c.uploaded, data)
	return string(data), nil
}

func (c *stubSourceClient) Download(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (c *stubSourceClient) Delete(_ context.Context, _ string) error { return nil }

func TestUploadFileChunksWithRoundRobinStrategy(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	s2 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	sources := []repository.Source{s1, s2}

	clients := map[primitive.ObjectID]*stubSourceClient{}
	factory := func(_ context.Context, src *repository.Source) (sourceClient, error) {
		cli := &stubSourceClient{}
		clients[src.ID] = cli
		return cli, nil
	}

	payload := []byte("abcdefghij")
	chunks, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader(payload), sources, 3, factory, &RoundRobinSelector{})
	if err != nil {
		t.Fatalf("upload chunks: %v", err)
	}
	if len(chunks) != 4 {
		t.Fatalf("unexpected chunk count: %d", len(chunks))
	}
	if chunks[0].SourceID != s1.ID || chunks[1].SourceID != s2.ID || chunks[2].SourceID != s1.ID || chunks[3].SourceID != s2.ID {
		t.Fatalf("chunks are not distributed in round-robin order: %+v", chunks)
	}
	if got := len(clients[s1.ID].uploaded); got != 2 {
		t.Fatalf("unexpected uploads for source1: %d", got)
	}
	if got := len(clients[s2.ID].uploaded); got != 2 {
		t.Fatalf("unexpected uploads for source2: %d", got)
	}
}
