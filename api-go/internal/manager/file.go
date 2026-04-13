package manager

import (
	"bytes"
	"context"
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
	}
	return nil
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

		chunks = append(chunks, repository.FileChunk{SourceID: src.ID, Name: chunkName, Order: idx, Size: int64(n)})
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
