package manager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/example/sfree/api-go/internal/repository"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrChecksumMismatch is returned when a downloaded chunk's SHA-256 hash does
// not match the value stored at upload time, indicating possible corruption.
var ErrChecksumMismatch = errors.New("checksum mismatch")

func StreamFile(ctx context.Context, srcRepo *repository.SourceRepository, f *repository.File, w io.Writer) error {
	return streamFileWithFactory(ctx, f, w, sourceClientFactoryFromRepository(srcRepo))
}

func StreamFileRange(ctx context.Context, srcRepo *repository.SourceRepository, f *repository.File, w io.Writer, start, end int64) error {
	return streamFileRangeWithFactory(ctx, f, w, start, end, sourceClientFactoryFromRepository(srcRepo))
}

// streamFileWithFactory is the testable core of StreamFile. The factory receives
// a Source stub containing only the SourceID; it is responsible for resolving the
// full source configuration and returning a ready client.
func streamFileWithFactory(ctx context.Context, f *repository.File, w io.Writer, factory SourceClientFactory) error {
	ctx, span := tracer.Start(ctx, "StreamFile",
		trace.WithAttributes(
			attribute.String("file.id", f.ID.Hex()),
			attribute.Int("file.chunks", len(f.Chunks)),
		),
	)
	defer span.End()

	clientCache := newSourceClientCache(factory)
	for i, ch := range f.Chunks {
		cli, err := clientCache.get(ctx, repository.Source{ID: ch.SourceID})
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "client creation failed")
			return err
		}

		_, chunkSpan := tracer.Start(ctx, "DownloadChunk",
			trace.WithAttributes(
				attribute.Int("chunk.order", i),
				attribute.String("chunk.source_id", ch.SourceID.Hex()),
			),
		)
		rc, err := cli.Download(ctx, ch.Name)
		if err != nil {
			chunkSpan.RecordError(err)
			chunkSpan.SetStatus(codes.Error, "download failed")
			chunkSpan.End()
			span.RecordError(err)
			span.SetStatus(codes.Error, "chunk download failed")
			return err
		}
		if ch.Checksum == "" {
			_, err = io.Copy(w, rc)
			_ = rc.Close()
			if err != nil {
				chunkSpan.RecordError(err)
				chunkSpan.SetStatus(codes.Error, "copy failed")
				chunkSpan.End()
				span.RecordError(err)
				span.SetStatus(codes.Error, "chunk copy failed")
				return err
			}
			chunkSpan.End()
			continue
		}
		chunkData, err := readChecksummedChunk(rc, ch, i)
		_ = rc.Close()
		if err != nil {
			chunkSpan.RecordError(err)
			chunkSpan.SetStatus(codes.Error, "read failed")
			chunkSpan.End()
			span.RecordError(err)
			span.SetStatus(codes.Error, "chunk read failed")
			return err
		}
		if _, err = w.Write(chunkData); err != nil {
			chunkSpan.RecordError(err)
			chunkSpan.SetStatus(codes.Error, "write failed")
			chunkSpan.End()
			span.RecordError(err)
			span.SetStatus(codes.Error, "chunk write failed")
			return err
		}
		chunkSpan.End()
	}
	return nil
}

func streamFileRangeWithFactory(ctx context.Context, f *repository.File, w io.Writer, start, end int64, factory SourceClientFactory) error {
	ctx, span := tracer.Start(ctx, "StreamFileRange",
		trace.WithAttributes(
			attribute.String("file.id", f.ID.Hex()),
			attribute.Int("file.chunks", len(f.Chunks)),
			attribute.Int64("range.start", start),
			attribute.Int64("range.end", end),
		),
	)
	defer span.End()

	clientCache := newSourceClientCache(factory)
	var offset int64
	for i, ch := range f.Chunks {
		chunkStart := offset
		chunkEnd := offset + ch.Size - 1
		offset += ch.Size
		if ch.Size <= 0 || end < chunkStart {
			break
		}
		if start > chunkEnd {
			continue
		}

		localStart := maxInt64(0, start-chunkStart)
		localEnd := minInt64(ch.Size-1, end-chunkStart)
		if localEnd < localStart {
			continue
		}

		cli, err := clientCache.get(ctx, repository.Source{ID: ch.SourceID})
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "client creation failed")
			return err
		}

		_, chunkSpan := tracer.Start(ctx, "DownloadChunkRange",
			trace.WithAttributes(
				attribute.Int("chunk.order", i),
				attribute.String("chunk.source_id", ch.SourceID.Hex()),
				attribute.Int64("chunk.range_start", localStart),
				attribute.Int64("chunk.range_end", localEnd),
			),
		)
		rc, err := cli.Download(ctx, ch.Name)
		if err != nil {
			chunkSpan.RecordError(err)
			chunkSpan.SetStatus(codes.Error, "download failed")
			chunkSpan.End()
			span.RecordError(err)
			span.SetStatus(codes.Error, "chunk download failed")
			return err
		}

		if ch.Checksum != "" {
			chunkData, err := readChecksummedChunk(rc, ch, i)
			_ = rc.Close()
			if err != nil {
				chunkSpan.RecordError(err)
				chunkSpan.SetStatus(codes.Error, "read failed")
				chunkSpan.End()
				span.RecordError(err)
				span.SetStatus(codes.Error, "chunk read failed")
				return err
			}
			if _, err = w.Write(chunkData[localStart : localEnd+1]); err != nil {
				chunkSpan.RecordError(err)
				chunkSpan.SetStatus(codes.Error, "write failed")
				chunkSpan.End()
				span.RecordError(err)
				span.SetStatus(codes.Error, "chunk write failed")
				return err
			}
			chunkSpan.End()
			continue
		}

		if localStart > 0 {
			if _, err = io.CopyN(io.Discard, rc, localStart); err != nil {
				_ = rc.Close()
				chunkSpan.RecordError(err)
				chunkSpan.SetStatus(codes.Error, "seek failed")
				chunkSpan.End()
				span.RecordError(err)
				span.SetStatus(codes.Error, "chunk seek failed")
				return err
			}
		}
		if _, err = io.CopyN(w, rc, localEnd-localStart+1); err != nil {
			_ = rc.Close()
			chunkSpan.RecordError(err)
			chunkSpan.SetStatus(codes.Error, "copy failed")
			chunkSpan.End()
			span.RecordError(err)
			span.SetStatus(codes.Error, "chunk copy failed")
			return err
		}
		_ = rc.Close()
		chunkSpan.End()
	}
	return nil
}

func readChecksummedChunk(r io.Reader, ch repository.FileChunk, order int) ([]byte, error) {
	if ch.Size < 0 {
		return nil, fmt.Errorf("%w: chunk %d invalid size", ErrChecksumMismatch, order)
	}
	data, err := io.ReadAll(io.LimitReader(r, ch.Size+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != ch.Size {
		return nil, fmt.Errorf("%w: chunk %d size", ErrChecksumMismatch, order)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != ch.Checksum {
		return nil, fmt.Errorf("%w: chunk %d", ErrChecksumMismatch, order)
	}
	return data, nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
