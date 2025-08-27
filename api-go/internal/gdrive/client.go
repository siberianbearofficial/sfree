package gdrive

import (
	"context"
	"io"

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
