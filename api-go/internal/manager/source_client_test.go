package manager

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/gdrive"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/s3compat"
	"github.com/example/sfree/api-go/internal/telegram"
)

type fakeFullGDriveClient struct {
	files        []gdrive.File
	total        int64
	used         int64
	free         int64
	downloadBody string
}

func (c *fakeFullGDriveClient) Upload(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (c *fakeFullGDriveClient) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(c.downloadBody)), nil
}

func (c *fakeFullGDriveClient) Delete(context.Context, string) error {
	return nil
}

func (c *fakeFullGDriveClient) ListFiles(context.Context) ([]gdrive.File, error) {
	return c.files, nil
}

func (c *fakeFullGDriveClient) StorageInfo(context.Context) (int64, int64, int64, error) {
	return c.total, c.used, c.free, nil
}

type fakeFullS3Client struct {
	objects      []s3compat.ObjectInfo
	used         int64
	downloadBody string
}

func (c *fakeFullS3Client) Upload(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (c *fakeFullS3Client) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(c.downloadBody)), nil
}

func (c *fakeFullS3Client) Delete(context.Context, string) error {
	return nil
}

func (c *fakeFullS3Client) ListObjects(context.Context) ([]s3compat.ObjectInfo, int64, error) {
	return c.objects, c.used, nil
}

func TestSourceClientBuilderInfoClients(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gdriveClient := &fakeFullGDriveClient{
		files: []gdrive.File{{ID: "file-1", Name: "doc.txt", Size: 12}},
		total: 100,
		used:  30,
		free:  70,
	}
	s3Client := &fakeFullS3Client{
		objects: []s3compat.ObjectInfo{{Key: "object.txt", Size: 9}},
		used:    9,
	}

	var gotGDriveKey string
	var gotS3Config s3compat.Config
	builder := sourceClientBuilder{
		newGDriveClient: func(_ context.Context, credsJSON []byte) (fullGDriveSourceClient, error) {
			gotGDriveKey = string(credsJSON)
			return gdriveClient, nil
		},
		newTelegramClient: func(telegram.Config) (sourceClient, error) {
			t.Fatal("telegram client should not be built for source info")
			return nil, nil
		},
		newS3Client: func(_ context.Context, cfg s3compat.Config) (fullS3SourceClient, error) {
			gotS3Config = cfg
			return s3Client, nil
		},
	}

	infoClient, err := builder.NewSourceInfoClient(ctx, &repository.Source{Type: repository.SourceTypeGDrive, Key: "gdrive-creds"})
	if err != nil {
		t.Fatalf("gdrive info client: %v", err)
	}
	info, err := infoClient.Info(ctx)
	if err != nil {
		t.Fatalf("gdrive info: %v", err)
	}
	if gotGDriveKey != "gdrive-creds" || len(info.Files) != 1 || info.Files[0].ID != "file-1" || info.StorageTotal != 100 || info.StorageUsed != 30 || info.StorageFree != 70 {
		t.Fatalf("unexpected gdrive info: key=%q info=%+v", gotGDriveKey, info)
	}

	s3Key, err := s3compat.EncodeConfig(s3compat.Config{
		Endpoint:     "http://s3.test",
		Bucket:       "bucket",
		AccessKeyID:  "access",
		SecretAccess: "secret",
	})
	if err != nil {
		t.Fatalf("encode s3 config: %v", err)
	}
	infoClient, err = builder.NewSourceInfoClient(ctx, &repository.Source{Type: repository.SourceTypeS3, Key: s3Key})
	if err != nil {
		t.Fatalf("s3 info client: %v", err)
	}
	info, err = infoClient.Info(ctx)
	if err != nil {
		t.Fatalf("s3 info: %v", err)
	}
	if gotS3Config.Region != "us-east-1" || len(info.Files) != 1 || info.Files[0].ID != "object.txt" || info.StorageUsed != 9 {
		t.Fatalf("unexpected s3 info: cfg=%+v info=%+v", gotS3Config, info)
	}

	infoClient, err = builder.NewSourceInfoClient(ctx, &repository.Source{Type: repository.SourceTypeTelegram, Key: "{}"})
	if err != nil {
		t.Fatalf("telegram info client: %v", err)
	}
	info, err = infoClient.Info(ctx)
	if err != nil {
		t.Fatalf("telegram info: %v", err)
	}
	if len(info.Files) != 0 || info.StorageTotal != 0 || info.StorageUsed != 0 || info.StorageFree != 0 {
		t.Fatalf("unexpected telegram info: %+v", info)
	}
}

func TestSourceClientBuilderDirectDownloadClients(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gdriveClient := &fakeFullGDriveClient{downloadBody: "gdrive-body"}
	s3Client := &fakeFullS3Client{downloadBody: "s3-body"}
	builder := sourceClientBuilder{
		newGDriveClient: func(context.Context, []byte) (fullGDriveSourceClient, error) {
			return gdriveClient, nil
		},
		newTelegramClient: func(telegram.Config) (sourceClient, error) {
			t.Fatal("telegram client should not be built for direct source downloads")
			return nil, nil
		},
		newS3Client: func(context.Context, s3compat.Config) (fullS3SourceClient, error) {
			return s3Client, nil
		},
	}

	downloadClient, err := builder.NewDirectSourceClient(ctx, &repository.Source{Type: repository.SourceTypeGDrive, Key: "{}"})
	if err != nil {
		t.Fatalf("gdrive download client: %v", err)
	}
	body, err := downloadClient.Download(ctx, "file-id")
	if err != nil {
		t.Fatalf("gdrive download: %v", err)
	}
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read gdrive body: %v", err)
	}
	_ = body.Close()
	if string(data) != "gdrive-body" {
		t.Fatalf("unexpected gdrive body: %q", string(data))
	}

	s3Key, err := s3compat.EncodeConfig(s3compat.Config{
		Endpoint:     "http://s3.test",
		Bucket:       "bucket",
		AccessKeyID:  "access",
		SecretAccess: "secret",
	})
	if err != nil {
		t.Fatalf("encode s3 config: %v", err)
	}
	downloadClient, err = builder.NewDirectSourceClient(ctx, &repository.Source{Type: repository.SourceTypeS3, Key: s3Key})
	if err != nil {
		t.Fatalf("s3 download client: %v", err)
	}
	body, err = downloadClient.Download(ctx, "object.txt")
	if err != nil {
		t.Fatalf("s3 download: %v", err)
	}
	data, err = io.ReadAll(body)
	if err != nil {
		t.Fatalf("read s3 body: %v", err)
	}
	_ = body.Close()
	if string(data) != "s3-body" {
		t.Fatalf("unexpected s3 body: %q", string(data))
	}

	_, err = builder.NewDirectSourceClient(ctx, &repository.Source{Type: repository.SourceTypeTelegram, Key: "{}"})
	if !errors.Is(err, ErrSourceDownloadUnsupported) {
		t.Fatalf("expected ErrSourceDownloadUnsupported, got %v", err)
	}
}

func TestSourceClientBuilderDirectDownloadKeepsStreamContextOpen(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	builder := sourceClientBuilder{
		newGDriveClient: func(context.Context, []byte) (fullGDriveSourceClient, error) {
			return &contextAwareFullSourceClient{payload: "streamed-body"}, nil
		},
	}

	downloadClient, err := builder.NewDirectSourceClient(ctx, &repository.Source{Type: repository.SourceTypeGDrive, Key: "{}"})
	if err != nil {
		t.Fatalf("gdrive download client: %v", err)
	}
	body, err := downloadClient.Download(ctx, "file-id")
	if err != nil {
		t.Fatalf("gdrive download: %v", err)
	}
	data, readErr := io.ReadAll(body)
	closeErr := body.Close()
	if readErr != nil {
		t.Fatalf("read gdrive body: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close gdrive body: %v", closeErr)
	}
	if string(data) != "streamed-body" {
		t.Fatalf("unexpected gdrive body: %q", string(data))
	}
}

type contextAwareFullSourceClient struct {
	payload string
}

func (c *contextAwareFullSourceClient) Upload(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (c *contextAwareFullSourceClient) Download(ctx context.Context, string) (io.ReadCloser, error) {
	return &contextAwareSourceBody{ctx: ctx, reader: strings.NewReader(c.payload)}, nil
}

func (c *contextAwareFullSourceClient) Delete(context.Context, string) error {
	return nil
}

func (c *contextAwareFullSourceClient) ListFiles(context.Context) ([]gdrive.File, error) {
	return nil, nil
}

func (c *contextAwareFullSourceClient) StorageInfo(context.Context) (int64, int64, int64, error) {
	return 0, 0, 0, nil
}

type contextAwareSourceBody struct {
	ctx    context.Context
	reader *strings.Reader
}

func (b *contextAwareSourceBody) Read(p []byte) (int, error) {
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	return b.reader.Read(p)
}

func (b *contextAwareSourceBody) Close() error {
	return nil
}
