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

	"github.com/example/sfree/api-go/internal/gdrive"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/resilience"
	"github.com/example/sfree/api-go/internal/s3compat"
	"github.com/example/sfree/api-go/internal/telegram"
	"github.com/example/sfree/api-go/internal/telemetry"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var ErrUnsupportedSourceType = errors.New("unsupported source type")

// ErrChecksumMismatch is returned when a downloaded chunk's SHA-256 hash does
// not match the value stored at upload time, indicating possible corruption.
var ErrChecksumMismatch = errors.New("checksum mismatch")

var tracer = telemetry.Tracer("sfree/manager")

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

// WeightedSelector distributes chunks across sources proportionally to their
// configured weights. It builds a repeating sequence where each source appears
// a number of times equal to its weight, then cycles through that sequence.
type WeightedSelector struct {
	sequence []int
	next     int
}

// NewWeightedSelector creates a selector that distributes chunks according to
// per-source weights. The weights map uses source ID hex strings as keys.
// Sources without an entry default to weight 1.
func NewWeightedSelector(sources []repository.Source, weights map[string]int) *WeightedSelector {
	seq := make([]int, 0)
	for i, src := range sources {
		w := weights[src.ID.Hex()]
		if w <= 0 {
			w = 1
		}
		for j := 0; j < w; j++ {
			seq = append(seq, i)
		}
	}
	return &WeightedSelector{sequence: seq}
}

func (s *WeightedSelector) NextSource(sources []repository.Source) (int, repository.Source, error) {
	if len(sources) == 0 || len(s.sequence) == 0 {
		return 0, repository.Source{}, errors.New("no sources")
	}
	idx := s.sequence[s.next%len(s.sequence)]
	s.next = (s.next + 1) % len(s.sequence)
	if idx >= len(sources) {
		return 0, repository.Source{}, errors.New("source index out of range")
	}
	return idx, sources[idx], nil
}

// SelectorForBucket returns the appropriate SourceSelector based on the
// bucket's configured distribution strategy.
func SelectorForBucket(bucket *repository.Bucket, sources []repository.Source) SourceSelector {
	switch bucket.DistributionStrategy {
	case repository.StrategyWeighted:
		return NewWeightedSelector(sources, bucket.SourceWeights)
	default:
		return &RoundRobinSelector{}
	}
}

// ResilienceConfig holds the timeout and circuit breaker settings applied
// to every source client. Zero values use sensible defaults (30s timeout,
// 5-failure threshold, 30s recovery).
var ResilienceConfig = resilience.DefaultWrapperConfig()

func NewSourceClient(ctx context.Context, src *repository.Source) (sourceClient, error) {
	if src == nil {
		return nil, errors.New("nil source")
	}
	var (
		cli sourceClient
		err error
	)
	switch src.Type {
	case repository.SourceTypeGDrive:
		cli, err = gdrive.NewClient(ctx, []byte(src.Key))
	case repository.SourceTypeTelegram:
		var tcfg telegram.Config
		tcfg, err = telegram.ParseConfig(src.Key)
		if err != nil {
			return nil, err
		}
		cli, err = telegram.NewClient(tcfg)
	case repository.SourceTypeS3:
		var scfg s3compat.Config
		scfg, err = s3compat.ParseConfig(src.Key)
		if err != nil {
			return nil, err
		}
		cli, err = s3compat.NewClient(ctx, scfg)
	default:
		return nil, ErrUnsupportedSourceType
	}
	if err != nil {
		return nil, err
	}
	return resilience.Wrap(cli, ResilienceConfig), nil
}

func StreamFile(ctx context.Context, srcRepo *repository.SourceRepository, f *repository.File, w io.Writer) error {
	// Wrap the real source repo lookup in a SourceClientFactory so the core
	// streaming logic can be tested without a live repository.
	factory := func(ctx context.Context, src *repository.Source) (sourceClient, error) {
		fullSrc, err := srcRepo.GetByID(ctx, src.ID)
		if err != nil {
			return nil, err
		}
		return NewSourceClient(ctx, fullSrc)
	}
	return streamFileWithFactory(ctx, f, w, factory)
}

func StreamFileRange(ctx context.Context, srcRepo *repository.SourceRepository, f *repository.File, w io.Writer, start, end int64) error {
	factory := func(ctx context.Context, src *repository.Source) (sourceClient, error) {
		fullSrc, err := srcRepo.GetByID(ctx, src.ID)
		if err != nil {
			return nil, err
		}
		return NewSourceClient(ctx, fullSrc)
	}
	return streamFileRangeWithFactory(ctx, f, w, start, end, factory)
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

	clients := make(map[primitive.ObjectID]sourceClient)
	for i, ch := range f.Chunks {
		cli, ok := clients[ch.SourceID]
		if !ok {
			var err error
			cli, err = factory(ctx, &repository.Source{ID: ch.SourceID})
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "client creation failed")
				return err
			}
			clients[ch.SourceID] = cli
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

	clients := make(map[primitive.ObjectID]sourceClient)
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

		cli, ok := clients[ch.SourceID]
		if !ok {
			var err error
			cli, err = factory(ctx, &repository.Source{ID: ch.SourceID})
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "client creation failed")
				return err
			}
			clients[ch.SourceID] = cli
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

func DeleteFileChunks(ctx context.Context, srcRepo *repository.SourceRepository, chunks []repository.FileChunk) error {
	ctx, span := tracer.Start(ctx, "DeleteFileChunks",
		trace.WithAttributes(
			attribute.Int("chunks.count", len(chunks)),
		),
	)
	defer span.End()

	clients := make(map[primitive.ObjectID]sourceClient)
	for _, ch := range chunks {
		cli, ok := clients[ch.SourceID]
		if !ok {
			src, err := srcRepo.GetByID(ctx, ch.SourceID)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "source lookup failed")
				return err
			}
			cli, err = NewSourceClient(ctx, src)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "client creation failed")
				return err
			}
			clients[ch.SourceID] = cli
		}
		if err := cli.Delete(ctx, ch.Name); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "delete failed")
			return err
		}
	}
	return nil
}

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
	failoverCount := 0
	for {
		n, readErr := r.Read(buf)
		if readErr != nil && readErr != io.EOF {
			span.RecordError(readErr)
			span.SetStatus(codes.Error, "read failed")
			return nil, readErr
		}
		if n == 0 {
			break
		}

		chunkData := make([]byte, n)
		copy(chunkData, buf[:n])

		src, cli, err := pickSourceClient(ctx, sources, selector, clients, factory)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "source selection failed")
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
				altSrc, altCli, altErr := pickSourceClient(ctx, sources, selector, clients, factory)
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
		if readErr == io.EOF {
			break
		}
	}
	span.SetAttributes(
		attribute.Int("chunks.uploaded", len(chunks)),
		attribute.Int("chunks.failovers", failoverCount),
	)
	return chunks, nil
}

// pickSourceClient selects the next source and returns its client, creating
// the client if needed. It uses the provided selector, client cache, and factory.
func pickSourceClient(ctx context.Context, sources []repository.Source, selector SourceSelector, clients map[primitive.ObjectID]sourceClient, factory SourceClientFactory) (repository.Source, sourceClient, error) {
	_, src, err := selector.NextSource(sources)
	if err != nil {
		return repository.Source{}, nil, err
	}
	cli, ok := clients[src.ID]
	if !ok {
		cli, err = factory(ctx, &src)
		if err != nil {
			return repository.Source{}, nil, err
		}
		clients[src.ID] = cli
	}
	return src, cli, nil
}
