package manager

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/example/s3aas/api-go/internal/gdrive"
	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/example/s3aas/api-go/internal/s3compat"
	"github.com/example/s3aas/api-go/internal/telegram"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrUnsupportedSourceType = errors.New("unsupported source type")

type sourceClient interface {
	Upload(ctx context.Context, name string, r io.Reader) (string, error)
	Download(ctx context.Context, name string) (io.ReadCloser, error)
	Delete(ctx context.Context, name string) error
}

type SourceClientFactory func(ctx context.Context, src *repository.Source) (sourceClient, error)

type SourceSelector interface {
	NextSource(sources []repository.Source) (int, repository.Source, error)
}

type RoundRobinSelector struct {
	next int
}

func (s *RoundRobinSelector) NextSource(sources []repository.Source) (int, repository.Source, error) {
	if len(sources) == 0 {
		return 0, repository.Source{}, errors.New("no sources")
	}
	idx := s.next % len(sources)
	s.next = (s.next + 1) % len(sources)
	return idx, sources[idx], nil
}

func NewSourceClient(ctx context.Context, src *repository.Source) (sourceClient, error) {
	if src == nil {
		return nil, errors.New("nil source")
	}
	switch src.Type {
	case repository.SourceTypeGDrive:
		return gdrive.NewClient(ctx, []byte(src.Key))
	case repository.SourceTypeTelegram:
		cfg, err := telegram.ParseConfig(src.Key)
		if err != nil {
			return nil, err
		}
		return telegram.NewClient(cfg)
	case repository.SourceTypeS3:
		cfg, err := s3compat.ParseConfig(src.Key)
		if err != nil {
			return nil, err
		}
		return s3compat.NewClient(ctx, cfg)
	default:
		return nil, ErrUnsupportedSourceType
	}
}

func StreamFile(ctx context.Context, srcRepo *repository.SourceRepository, f *repository.File, w io.Writer) error {
	clients := make(map[primitive.ObjectID]sourceClient)
	for _, ch := range f.Chunks {
		cli, ok := clients[ch.SourceID]
		if !ok {
			src, err := srcRepo.GetByID(ctx, ch.SourceID)
			if err != nil {
				return err
			}
			cli, err = NewSourceClient(ctx, src)
			if err != nil {
				return err
			}
			clients[ch.SourceID] = cli
		}
		rc, err := cli.Download(ctx, ch.Name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, rc); err != nil {
			_ = rc.Close()
			return err
		}
		_ = rc.Close()
	}
	return nil
}

func DeleteFileChunks(ctx context.Context, srcRepo *repository.SourceRepository, chunks []repository.FileChunk) error {
	clients := make(map[primitive.ObjectID]sourceClient)
	for _, ch := range chunks {
		cli, ok := clients[ch.SourceID]
		if !ok {
			src, err := srcRepo.GetByID(ctx, ch.SourceID)
			if err != nil {
				return err
			}
			cli, err = NewSourceClient(ctx, src)
			if err != nil {
				return err
			}
			clients[ch.SourceID] = cli
		}
		if err := cli.Delete(ctx, ch.Name); err != nil {
			return err
		}
	}
	return nil
}

func UploadFileChunks(ctx context.Context, r io.Reader, sources []repository.Source, chunkSize int) ([]repository.FileChunk, error) {
	return UploadFileChunksWithStrategy(ctx, r, sources, chunkSize, NewSourceClient, &RoundRobinSelector{})
}

func UploadFileChunksWithStrategy(ctx context.Context, r io.Reader, sources []repository.Source, chunkSize int, factory SourceClientFactory, selector SourceSelector) ([]repository.FileChunk, error) {
	if len(sources) == 0 {
		return nil, errors.New("no sources")
	}
	if factory == nil {
		factory = NewSourceClient
	}
	if selector == nil {
		selector = &RoundRobinSelector{}
	}
	if chunkSize <= 0 {
		chunkSize = 5 * 1024 * 1024
	}
	clients := make(map[primitive.ObjectID]sourceClient)
	chunks := make([]repository.FileChunk, 0)
	buf := make([]byte, chunkSize)
	idx := 0
	for {
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}
		_, src, err := selector.NextSource(sources)
		if err != nil {
			return nil, err
		}
		cli, ok := clients[src.ID]
		if !ok {
			cli, err = factory(ctx, &src)
			if err != nil {
				return nil, err
			}
			clients[src.ID] = cli
		}
		driveName := primitive.NewObjectID().Hex()
		chunkName, err := cli.Upload(ctx, driveName, bytes.NewReader(buf[:n]))
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, repository.FileChunk{SourceID: src.ID, Name: chunkName, Order: idx, Size: int64(n)})
		idx++
		if err == io.EOF {
			break
		}
	}
	return chunks, nil
}
