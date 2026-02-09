package manager

import (
	"context"
	"io"

	"github.com/example/s3aas/api-go/internal/gdrive"
	"github.com/example/s3aas/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// StreamFile downloads file chunks from sources and writes them to w.
// It returns an error if any chunk cannot be downloaded or written.
func StreamFile(ctx context.Context, srcRepo *repository.SourceRepository, f *repository.File, w io.Writer) error {
	clients := make(map[primitive.ObjectID]*gdrive.Client)
	for _, ch := range f.Chunks {
		cli, ok := clients[ch.SourceID]
		if !ok {
			src, err := srcRepo.GetByID(ctx, ch.SourceID)
			if err != nil {
				return err
			}
			cli, err = gdrive.NewClient(ctx, []byte(src.Key))
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
	clients := make(map[primitive.ObjectID]*gdrive.Client)
	for _, ch := range chunks {
		cli, ok := clients[ch.SourceID]
		if !ok {
			src, err := srcRepo.GetByID(ctx, ch.SourceID)
			if err != nil {
				return err
			}
			cli, err = gdrive.NewClient(ctx, []byte(src.Key))
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
