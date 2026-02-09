package manager

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/example/s3aas/api-go/internal/gdrive"
	"github.com/example/s3aas/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrUnsupportedSourceType = errors.New("unsupported source type")

type sourceClient interface {
	Upload(ctx context.Context, name string, r io.Reader) (string, error)
	Download(ctx context.Context, name string) (io.ReadCloser, error)
	Delete(ctx context.Context, name string) error
}

func newSourceClient(ctx context.Context, src *repository.Source) (sourceClient, error) {
	if src == nil {
		return nil, errors.New("nil source")
	}
	switch src.Type {
	case repository.SourceTypeGDrive:
		return gdrive.NewClient(ctx, []byte(src.Key))
	default:
		return nil, ErrUnsupportedSourceType
	}
}

// StreamFile downloads file chunks from sources and writes them to w.
// It returns an error if any chunk cannot be downloaded or written.
func StreamFile(ctx context.Context, srcRepo *repository.SourceRepository, f *repository.File, w io.Writer) error {
	clients := make(map[primitive.ObjectID]sourceClient)
	for _, ch := range f.Chunks {
		cli, ok := clients[ch.SourceID]
		if !ok {
			src, err := srcRepo.GetByID(ctx, ch.SourceID)
			if err != nil {
				return err
			}
			cli, err = newSourceClient(ctx, src)
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
			cli, err = newSourceClient(ctx, src)
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
	if len(sources) == 0 {
		return nil, errors.New("no sources")
	}
	if chunkSize <= 0 {
		chunkSize = 5 * 1024 * 1024
	}
	clients := make([]sourceClient, len(sources))
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
		src := sources[idx%len(sources)]
		if clients[idx%len(sources)] == nil {
			cli, err := newSourceClient(ctx, &src)
			if err != nil {
				return nil, err
			}
			clients[idx%len(sources)] = cli
		}
		driveName := primitive.NewObjectID().Hex()
		chunkName, err := clients[idx%len(sources)].Upload(ctx, driveName, bytes.NewReader(buf[:n]))
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
