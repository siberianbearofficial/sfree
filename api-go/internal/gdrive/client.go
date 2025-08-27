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
