package manager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type stubSourceClient struct {
	uploaded    [][]byte
	uploadErr   error
	downloadErr error
	deleteErr   error
}

func (c *stubSourceClient) Upload(_ context.Context, _ string, r io.Reader) (string, error) {
	if c.uploadErr != nil {
		return "", c.uploadErr
	}
	data, _ := io.ReadAll(r)
	c.uploaded = append(c.uploaded, data)
	return string(data), nil
}

func (c *stubSourceClient) Download(_ context.Context, name string) (io.ReadCloser, error) {
	if c.downloadErr != nil {
		return nil, c.downloadErr
	}
	return io.NopCloser(bytes.NewReader([]byte(name))), nil
}

func (c *stubSourceClient) Delete(_ context.Context, _ string) error { return c.deleteErr }

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

func TestRoundRobinSelectorNoSources(t *testing.T) {
	t.Parallel()
	sel := &RoundRobinSelector{}
	_, _, err := sel.NextSource(nil)
	if err == nil {
		t.Fatal("expected error for empty sources, got nil")
	}
	_, _, err = sel.NextSource([]repository.Source{})
	if err == nil {
		t.Fatal("expected error for empty sources slice, got nil")
	}
}

func TestRoundRobinSelectorAdvances(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID()}
	s2 := repository.Source{ID: primitive.NewObjectID()}
	sources := []repository.Source{s1, s2}
	sel := &RoundRobinSelector{}

	idxA, srcA, _ := sel.NextSource(sources)
	idxB, srcB, _ := sel.NextSource(sources)
	idxC, srcC, _ := sel.NextSource(sources)

	if idxA != 0 || srcA.ID != s1.ID {
		t.Fatalf("first call: expected index 0 / s1, got %d / %v", idxA, srcA.ID)
	}
	if idxB != 1 || srcB.ID != s2.ID {
		t.Fatalf("second call: expected index 1 / s2, got %d / %v", idxB, srcB.ID)
	}
	if idxC != 0 || srcC.ID != s1.ID {
		t.Fatalf("third call: expected wrap to index 0 / s1, got %d / %v", idxC, srcC.ID)
	}
}

func TestUploadFileChunksNoSources(t *testing.T) {
	t.Parallel()
	_, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader([]byte("data")), nil, 4, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &stubSourceClient{}, nil
	}, &RoundRobinSelector{})
	if err == nil {
		t.Fatal("expected error for no sources")
	}
}

func TestUploadFileChunksFactoryError(t *testing.T) {
	t.Parallel()
	factoryErr := errors.New("factory failed")
	src := repository.Source{ID: primitive.NewObjectID()}
	_, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader([]byte("hello")), []repository.Source{src}, 10, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return nil, factoryErr
	}, &RoundRobinSelector{})
	if !errors.Is(err, factoryErr) {
		t.Fatalf("expected factory error, got %v", err)
	}
}

func TestUploadFileChunksUploadError(t *testing.T) {
	t.Parallel()
	uploadErr := errors.New("upload failed")
	src := repository.Source{ID: primitive.NewObjectID()}
	stub := &stubSourceClient{uploadErr: uploadErr}
	_, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader([]byte("hello")), []repository.Source{src}, 10, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return stub, nil
	}, &RoundRobinSelector{})
	if !errors.Is(err, uploadErr) {
		t.Fatalf("expected upload error, got %v", err)
	}
}

func TestNewSourceClientNilSource(t *testing.T) {
	t.Parallel()
	_, err := NewSourceClient(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil source")
	}
}

func TestNewSourceClientUnsupportedType(t *testing.T) {
	t.Parallel()
	src := &repository.Source{Type: "unsupported_type_xyz"}
	_, err := NewSourceClient(context.Background(), src)
	if !errors.Is(err, ErrUnsupportedSourceType) {
		t.Fatalf("expected ErrUnsupportedSourceType, got %v", err)
	}
}

func TestStreamFileNoChunks(t *testing.T) {
	t.Parallel()
	f := &repository.File{Chunks: []repository.FileChunk{}}
	var buf bytes.Buffer
	if err := StreamFile(context.Background(), nil, f, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty output, got %d bytes", buf.Len())
	}
}

func TestDeleteFileChunksNoChunks(t *testing.T) {
	t.Parallel()
	if err := DeleteFileChunks(context.Background(), nil, []repository.FileChunk{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
