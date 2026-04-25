package manager

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/sourcecap"
)

type bareSourceClient struct{}

func (bareSourceClient) Upload(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (bareSourceClient) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("download")), nil
}

func (bareSourceClient) Delete(context.Context, string) error {
	return nil
}

type streamingSourceClient struct {
	downloadCalls       int
	downloadStreamCalls int
}

func (c *streamingSourceClient) Upload(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (c *streamingSourceClient) Download(context.Context, string) (io.ReadCloser, error) {
	c.downloadCalls++
	return io.NopCloser(strings.NewReader("download")), nil
}

func (c *streamingSourceClient) DownloadStream(context.Context, string) (io.ReadCloser, error) {
	c.downloadStreamCalls++
	return io.NopCloser(strings.NewReader("stream")), nil
}

func (c *streamingSourceClient) Delete(context.Context, string) error {
	return nil
}

func TestInspectSourceProviderMapsCapabilityInfo(t *testing.T) {
	t.Parallel()

	info, err := inspectSourceProvider(context.Background(), &stubSourceClient{
		sourceInfo: sourcecap.Info{
			Files:        []sourcecap.File{{ID: "file-id", Name: "file.txt", Size: 42}},
			StorageTotal: 100,
			StorageUsed:  42,
			StorageFree:  58,
		},
	})
	if err != nil {
		t.Fatalf("inspect source provider: %v", err)
	}
	if len(info.Files) != 1 || info.Files[0].ID != "file-id" || info.Files[0].Name != "file.txt" || info.Files[0].Size != 42 {
		t.Fatalf("unexpected files: %#v", info.Files)
	}
	if info.StorageTotal != 100 || info.StorageUsed != 42 || info.StorageFree != 58 {
		t.Fatalf("unexpected storage info: %#v", info)
	}
}

func TestInspectSourceProviderRejectsMissingInfoCapability(t *testing.T) {
	t.Parallel()

	_, err := inspectSourceProvider(context.Background(), bareSourceClient{})
	if err != ErrUnsupportedSourceType {
		t.Fatalf("expected ErrUnsupportedSourceType, got %v", err)
	}
}

func TestProbeSourceHealthProviderRejectsMissingHealthCapability(t *testing.T) {
	t.Parallel()

	_, err := probeSourceHealthProvider(context.Background(), bareSourceClient{})
	if err != sourcecap.ErrUnsupportedCapability {
		t.Fatalf("expected ErrUnsupportedCapability, got %v", err)
	}
}

func TestDownloadSourceProviderFilePrefersStreamCapability(t *testing.T) {
	t.Parallel()

	cli := &streamingSourceClient{}
	body, err := downloadSourceProviderFile(context.Background(), cli, "file-id")
	if err != nil {
		t.Fatalf("download source provider file: %v", err)
	}
	defer func() { _ = body.Close() }()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(data) != "stream" {
		t.Fatalf("expected stream body, got %q", data)
	}
	if cli.downloadCalls != 0 {
		t.Fatalf("expected Download to be skipped, got %d calls", cli.downloadCalls)
	}
	if cli.downloadStreamCalls != 1 {
		t.Fatalf("expected DownloadStream once, got %d calls", cli.downloadStreamCalls)
	}
}
