package manager

import (
	"context"

	"github.com/example/sfree/api-go/internal/repository"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func DeleteFileChunks(ctx context.Context, srcRepo *repository.SourceRepository, chunks []repository.FileChunk) error {
	return DeleteFileChunksWithFactory(ctx, srcRepo, chunks, nil)
}

func DeleteFileChunksWithFactory(ctx context.Context, srcRepo *repository.SourceRepository, chunks []repository.FileChunk, factory SourceClientFactory) error {
	return deleteFileChunksWithFactory(ctx, chunks, sourceClientFactoryFromRepository(srcRepo, factory))
}

func deleteFileChunksWithFactory(ctx context.Context, chunks []repository.FileChunk, factory SourceClientFactory) error {
	ctx, span := tracer.Start(ctx, "DeleteFileChunks",
		trace.WithAttributes(
			attribute.Int("chunks.count", len(chunks)),
		),
	)
	defer span.End()

	clientCache := newSourceClientCache(factory)
	for _, ch := range chunks {
		cli, err := clientCache.get(ctx, repository.Source{ID: ch.SourceID})
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "client creation failed")
			return err
		}
		if err := cli.Delete(ctx, ch.Name); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "delete failed")
			return err
		}
	}
	return nil
}
