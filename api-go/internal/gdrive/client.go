package gdrive

import (
	"context"
	"io"

	"github.com/example/sfree/api-go/internal/sourcecap"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type Client struct {
	service *drive.Service
}

type File struct {
	ID   string
	Name string
	Size int64
}

func NewClient(ctx context.Context, credsJSON []byte) (*Client, error) {
	srv, err := drive.NewService(ctx, option.WithCredentialsJSON(credsJSON))
	if err != nil {
		return nil, err
	}
	return &Client{service: srv}, nil
}

func (c *Client) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	f := &drive.File{Name: name}
	created, err := c.service.Files.Create(f).Media(r).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return created.Id, nil
}

func (c *Client) Download(ctx context.Context, id string) (io.ReadCloser, error) {
	res, err := c.service.Files.Get(id).Context(ctx).Download()
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

func (c *Client) Delete(ctx context.Context, id string) error {
	return c.service.Files.Delete(id).Context(ctx).Do()
}

func (c *Client) ListFiles(ctx context.Context) ([]File, error) {
	var all []File
	pageToken := ""
	for {
		call := c.service.Files.List().Fields("nextPageToken,files(id,name,size)").PageSize(1000)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		res, err := call.Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		for _, f := range res.Files {
			all = append(all, File{ID: f.Id, Name: f.Name, Size: f.Size})
		}
		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}
	return all, nil
}

func (c *Client) StorageInfo(ctx context.Context) (total, used, free int64, err error) {
	about, err := c.service.About.Get().Fields("storageQuota").Context(ctx).Do()
	if err != nil {
		return 0, 0, 0, err
	}
	total = about.StorageQuota.Limit
	used = about.StorageQuota.Usage
	free = total - used
	return
}

func (c *Client) SourceInfo(ctx context.Context) (sourcecap.Info, error) {
	files, err := c.ListFiles(ctx)
	if err != nil {
		return sourcecap.Info{}, err
	}
	total, used, free, err := c.StorageInfo(ctx)
	if err != nil {
		return sourcecap.Info{}, err
	}
	respFiles := make([]sourcecap.File, 0, len(files))
	for _, f := range files {
		respFiles = append(respFiles, sourcecap.File{ID: f.ID, Name: f.Name, Size: f.Size})
	}
	return sourcecap.Info{Files: respFiles, StorageTotal: total, StorageUsed: used, StorageFree: free}, nil
}

func (c *Client) ProbeSourceHealth(ctx context.Context) (sourcecap.Health, error) {
	total, used, free, err := c.StorageInfo(ctx)
	if err != nil {
		return sourcecap.Health{
			Status:     sourcecap.HealthUnhealthy,
			ReasonCode: "probe_failed",
			Message:    "Google Drive metadata probe failed.",
		}, err
	}
	health := sourcecap.Health{
		Status:     sourcecap.HealthHealthy,
		ReasonCode: "ok",
		Message:    "Google Drive metadata is reachable.",
	}
	if total > 0 {
		health.Quota = sourcecap.Quota{TotalBytes: &total, UsedBytes: &used, FreeBytes: &free}
		if free <= 0 {
			health.Status = sourcecap.HealthUnhealthy
			health.ReasonCode = "quota_exhausted"
			health.Message = "Google Drive quota is exhausted."
			return health, nil
		}
		if free*100/total < 5 {
			health.Status = sourcecap.HealthDegraded
			health.ReasonCode = "quota_low"
			health.Message = "Google Drive quota is nearly exhausted."
			return health, nil
		}
	}
	return health, nil
}
