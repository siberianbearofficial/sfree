package manager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func UploadFileChunks(ctx context.Context, r io.Reader, sources []repository.Source, chunkSize int) ([]repository.FileChunk, error) {
	return UploadFileChunksWithStrategy(ctx, r, sources, chunkSize, NewSourceClient, &RoundRobinSelector{})
}

func UploadFileChunksWithStrategy(ctx context.Context, r io.Reader, sources []repository.Source, chunkSize int, factory SourceClientFactory, selector SourceSelector) ([]repository.FileChunk, error) {
	ctx, span := tracer.Start(ctx, "UploadFileChunks",
		trace.WithAttributes(
			attribute.Int("sources.count", len(sources)),
			attribute.Int("chunk.size", chunkSize),
		),
	)
	defer span.End()

	if len(sources) == 0 {
		return nil, errors.New("no sources")
	}
	if selector == nil {
		selector = &RoundRobinSelector{}
	}
	if chunkSize <= 0 {
		chunkSize = 5 * 1024 * 1024
	}
	clientCache := newSourceClientCache(factory)
	chunks := make([]repository.FileChunk, 0)
	buf := make([]byte, chunkSize)
	idx := 0
	failoverCount := 0
	for {
		n, readErr := io.ReadFull(r, buf)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			span.RecordError(readErr)
			span.SetStatus(codes.Error, "read failed")
			cleanupUploadedChunks(ctx, chunks, clientCache)
			return nil, readErr
		}
		if n == 0 {
			break
		}

		chunkData := make([]byte, n)
		copy(chunkData, buf[:n])

		src, cli, err := pickSourceClient(ctx, sources, selector, clientCache)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "source selection failed")
			cleanupUploadedChunks(ctx, chunks, clientCache)
			return nil, err
		}

		_, chunkSpan := tracer.Start(ctx, "UploadChunk",
			trace.WithAttributes(
				attribute.Int("chunk.order", idx),
				attribute.String("chunk.source_type", string(src.Type)),
				attribute.Int("chunk.bytes", n),
			),
		)
		driveName := primitive.NewObjectID().Hex()
		chunkName, uploadErr := cli.Upload(ctx, driveName, bytes.NewReader(chunkData))

		// Failover: on upload failure, try other sources.
		if uploadErr != nil && len(sources) > 1 {
			chunkSpan.RecordError(uploadErr)
			tried := map[primitive.ObjectID]bool{src.ID: true}
			for attempt := 0; attempt < len(sources)-1; attempt++ {
				altSrc, altCli, altErr := pickSourceClient(ctx, sources, selector, clientCache)
				if altErr != nil {
					continue
				}
				if tried[altSrc.ID] {
					continue
				}
				tried[altSrc.ID] = true
				failoverCount++
				slog.WarnContext(ctx, "upload failover",
					slog.Int("chunk.order", idx),
					slog.String("failed_source", src.ID.Hex()),
					slog.String("failover_source", altSrc.ID.Hex()),
					slog.String("last_error", uploadErr.Error()),
				)
				chunkSpan.AddEvent("failover",
					trace.WithAttributes(
						attribute.String("failover.from", src.ID.Hex()),
						attribute.String("failover.to", altSrc.ID.Hex()),
					),
				)
				altDriveName := primitive.NewObjectID().Hex()
				altName, altUpErr := altCli.Upload(ctx, altDriveName, bytes.NewReader(chunkData))
				if altUpErr == nil {
					src = altSrc
					chunkName = altName
					uploadErr = nil
					break
				}
				uploadErr = altUpErr
			}
		}

		if uploadErr != nil {
			chunkSpan.RecordError(uploadErr)
			chunkSpan.SetStatus(codes.Error, "upload failed")
			chunkSpan.End()
			span.RecordError(uploadErr)
			span.SetStatus(codes.Error, fmt.Sprintf("chunk %d upload failed on all sources", idx))
			cleanupUploadedChunks(ctx, chunks, clientCache)
			return nil, uploadErr
		}
		chunkSpan.End()

		sum := sha256.Sum256(chunkData)
		chunks = append(chunks, repository.FileChunk{
			SourceID: src.ID,
			Name:     chunkName,
			Order:    idx,
			Size:     int64(n),
			Checksum: hex.EncodeToString(sum[:]),
		})
		idx++
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	span.SetAttributes(
		attribute.Int("chunks.uploaded", len(chunks)),
		attribute.Int("chunks.failovers", failoverCount),
	)
	return chunks, nil
}

func cleanupUploadedChunks(ctx context.Context, chunks []repository.FileChunk, clientCache *sourceClientCache) {
	for _, ch := range chunks {
		cli, err := clientCache.get(ctx, repository.Source{ID: ch.SourceID})
		if err != nil {
			slog.WarnContext(ctx, "upload cleanup: get client", slog.String("error", err.Error()))
			continue
		}
		if err := cli.Delete(ctx, ch.Name); err != nil {
			slog.WarnContext(ctx, "upload cleanup: delete chunk",
				slog.String("source_id", ch.SourceID.Hex()),
				slog.String("chunk", ch.Name),
				slog.String("error", err.Error()),
			)
		}
	}
}

func pickSourceClient(ctx context.Context, sources []repository.Source, selector SourceSelector, clientCache *sourceClientCache) (repository.Source, sourceClient, error) {
	_, src, err := selector.NextSource(sources)
	if err != nil {
		return repository.Source{}, nil, err
	}
	cli, err := clientCache.get(ctx, src)
	if err != nil {
		return repository.Source{}, nil, err
	}
	return src, cli, nil
}
