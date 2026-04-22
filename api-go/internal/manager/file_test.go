package manager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"github.com/example/sfree/api-go/internal/gdrive"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/resilience"
	"github.com/example/sfree/api-go/internal/s3compat"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type stubSourceClient struct {
	uploaded     [][]byte
	uploadErr    error
	uploadErrAt  int
	uploadCalls  int
	downloadErr  error
	deleted      []string
	deleteErr    error
	gdriveFiles  []gdrive.File
	storageTotal int64
	storageUsed  int64
	storageFree  int64
	s3Objects    []s3compat.ObjectInfo
	s3Used       int64
}

type readOnceThenError struct {
	data []byte
	err  error
	done bool
}

func (r *readOnceThenError) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), nil
}

func (c *stubSourceClient) Upload(_ context.Context, _ string, r io.Reader) (string, error) {
	if c.uploadErr != nil && c.uploadCalls >= c.uploadErrAt {
		c.uploadCalls++
		return "", c.uploadErr
	}
	data, _ := io.ReadAll(r)
	c.uploadCalls++
	c.uploaded = append(c.uploaded, data)
	return string(data), nil
}

func (c *stubSourceClient) Download(_ context.Context, name string) (io.ReadCloser, error) {
	if c.downloadErr != nil {
		return nil, c.downloadErr
	}
	return io.NopCloser(bytes.NewReader([]byte(name))), nil
}

func (c *stubSourceClient) Delete(_ context.Context, name string) error {
	c.deleted = append(c.deleted, name)
	return c.deleteErr
}

func (c *stubSourceClient) ListFiles(_ context.Context) ([]gdrive.File, error) {
	return c.gdriveFiles, nil
}

func (c *stubSourceClient) StorageInfo(_ context.Context) (int64, int64, int64, error) {
	return c.storageTotal, c.storageUsed, c.storageFree, nil
}

func (c *stubSourceClient) ListObjects(_ context.Context) ([]s3compat.ObjectInfo, int64, error) {
	return c.s3Objects, c.s3Used, nil
}

func TestSourceClientCacheReusesClients(t *testing.T) {
	t.Parallel()
	srcID := primitive.NewObjectID()
	calls := 0
	cache := newSourceClientCache(func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		calls++
		return &stubSourceClient{}, nil
	})

	first, err := cache.get(context.Background(), repository.Source{ID: srcID})
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	second, err := cache.get(context.Background(), repository.Source{ID: srcID})
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	if first != second {
		t.Fatal("expected cached client to be reused")
	}
	if calls != 1 {
		t.Fatalf("expected factory to be called once, got %d", calls)
	}
}

func TestUploadFileChunksWithRoundRobinStrategy(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	s2 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	sources := []repository.Source{s1, s2}

	clients := map[primitive.ObjectID]*stubSourceClient{}
	factoryCalls := map[primitive.ObjectID]int{}
	factory := func(_ context.Context, src *repository.Source) (sourceClient, error) {
		factoryCalls[src.ID]++
		cli := &stubSourceClient{}
		clients[src.ID] = cli
		return cli, nil
	}

	payload := []byte("abcdefghij")
	chunks, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader(payload), sources, 3, factory, &RoundRobinSelector{})
	if err != nil {
		t.Fatalf("upload chunks: %v", err)
	}
	if len(chunks) != 4 {
		t.Fatalf("unexpected chunk count: %d", len(chunks))
	}
	if chunks[0].SourceID != s1.ID || chunks[1].SourceID != s2.ID || chunks[2].SourceID != s1.ID || chunks[3].SourceID != s2.ID {
		t.Fatalf("chunks are not distributed in round-robin order: %+v", chunks)
	}
	if got := len(clients[s1.ID].uploaded); got != 2 {
		t.Fatalf("unexpected uploads for source1: %d", got)
	}
	if got := len(clients[s2.ID].uploaded); got != 2 {
		t.Fatalf("unexpected uploads for source2: %d", got)
	}
	if factoryCalls[s1.ID] != 1 || factoryCalls[s2.ID] != 1 {
		t.Fatalf("expected one client per source, got calls: %+v", factoryCalls)
	}
}

type shortReadReader struct {
	data    []byte
	maxRead int
}

func (r *shortReadReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	if len(p) > r.maxRead {
		p = p[:r.maxRead]
	}
	if len(p) > len(r.data) {
		p = p[:len(r.data)]
	}
	n := copy(p, r.data[:len(p)])
	r.data = r.data[n:]
	return n, nil
}

func TestUploadFileChunksFillsChunksAcrossShortReads(t *testing.T) {
	t.Parallel()
	src := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	stub := &stubSourceClient{}
	factory := func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return stub, nil
	}

	payload := []byte("abcdefghij")
	chunks, err := UploadFileChunksWithStrategy(context.Background(), &shortReadReader{data: payload, maxRead: 1}, []repository.Source{src}, 4, factory, &RoundRobinSelector{})
	if err != nil {
		t.Fatalf("upload chunks: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(stub.uploaded) != 3 {
		t.Fatalf("expected 3 uploaded payloads, got %d", len(stub.uploaded))
	}

	wantUploads := [][]byte{payload[:4], payload[4:8], payload[8:]}
	for i, want := range wantUploads {
		if !bytes.Equal(stub.uploaded[i], want) {
			t.Fatalf("upload %d: got %q, want %q", i, stub.uploaded[i], want)
		}
		if chunks[i].Size != int64(len(want)) {
			t.Fatalf("chunk %d size: got %d, want %d", i, chunks[i].Size, len(want))
		}
	}
}

func TestRoundRobinSelectorNoSources(t *testing.T) {
	t.Parallel()
	sel := &RoundRobinSelector{}
	_, _, err := sel.NextSource(nil)
	if err == nil {
		t.Fatal("expected error for empty sources, got nil")
	}
	_, _, err = sel.NextSource([]repository.Source{})
	if err == nil {
		t.Fatal("expected error for empty sources slice, got nil")
	}
}

func TestRoundRobinSelectorAdvances(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID()}
	s2 := repository.Source{ID: primitive.NewObjectID()}
	sources := []repository.Source{s1, s2}
	sel := &RoundRobinSelector{}

	idxA, srcA, _ := sel.NextSource(sources)
	idxB, srcB, _ := sel.NextSource(sources)
	idxC, srcC, _ := sel.NextSource(sources)

	if idxA != 0 || srcA.ID != s1.ID {
		t.Fatalf("first call: expected index 0 / s1, got %d / %v", idxA, srcA.ID)
	}
	if idxB != 1 || srcB.ID != s2.ID {
		t.Fatalf("second call: expected index 1 / s2, got %d / %v", idxB, srcB.ID)
	}
	if idxC != 0 || srcC.ID != s1.ID {
		t.Fatalf("third call: expected wrap to index 0 / s1, got %d / %v", idxC, srcC.ID)
	}
}

func TestUploadFileChunksNoSources(t *testing.T) {
	t.Parallel()
	_, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader([]byte("data")), nil, 4, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &stubSourceClient{}, nil
	}, &RoundRobinSelector{})
	if err == nil {
		t.Fatal("expected error for no sources")
	}
}

func TestUploadFileChunksFactoryError(t *testing.T) {
	t.Parallel()
	factoryErr := errors.New("factory failed")
	src := repository.Source{ID: primitive.NewObjectID()}
	_, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader([]byte("hello")), []repository.Source{src}, 10, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return nil, factoryErr
	}, &RoundRobinSelector{})
	if !errors.Is(err, factoryErr) {
		t.Fatalf("expected factory error, got %v", err)
	}
}

func TestUploadFileChunksUploadError(t *testing.T) {
	t.Parallel()
	uploadErr := errors.New("upload failed")
	src := repository.Source{ID: primitive.NewObjectID()}
	stub := &stubSourceClient{uploadErr: uploadErr}
	_, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader([]byte("hello")), []repository.Source{src}, 10, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return stub, nil
	}, &RoundRobinSelector{})
	if !errors.Is(err, uploadErr) {
		t.Fatalf("expected upload error, got %v", err)
	}
}

func TestUploadFileChunksCleansUploadedChunksOnLaterUploadError(t *testing.T) {
	t.Parallel()
	uploadErr := errors.New("upload failed")
	src := repository.Source{ID: primitive.NewObjectID()}
	stub := &stubSourceClient{uploadErr: uploadErr, uploadErrAt: 1}

	chunks, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader([]byte("abcdef")), []repository.Source{src}, 3, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return stub, nil
	}, &RoundRobinSelector{})
	if !errors.Is(err, uploadErr) {
		t.Fatalf("expected upload error, got %v", err)
	}
	if chunks != nil {
		t.Fatalf("expected no chunks returned, got %+v", chunks)
	}
	if len(stub.deleted) != 1 || stub.deleted[0] != "abc" {
		t.Fatalf("expected uploaded chunk cleanup, got %+v", stub.deleted)
	}
}

func TestUploadFileChunksCleansUploadedChunksOnLaterReadError(t *testing.T) {
	t.Parallel()
	readErr := errors.New("read failed")
	src := repository.Source{ID: primitive.NewObjectID()}
	stub := &stubSourceClient{}
	reader := &readOnceThenError{data: []byte("abc"), err: readErr}

	chunks, err := UploadFileChunksWithStrategy(context.Background(), reader, []repository.Source{src}, 3, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return stub, nil
	}, &RoundRobinSelector{})
	if !errors.Is(err, readErr) {
		t.Fatalf("expected read error, got %v", err)
	}
	if chunks != nil {
		t.Fatalf("expected no chunks returned, got %+v", chunks)
	}
	if len(stub.deleted) != 1 || stub.deleted[0] != "abc" {
		t.Fatalf("expected uploaded chunk cleanup, got %+v", stub.deleted)
	}
}

func TestWeightedSelectorDistribution(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID()}
	s2 := repository.Source{ID: primitive.NewObjectID()}
	sources := []repository.Source{s1, s2}
	weights := map[string]int{s1.ID.Hex(): 3, s2.ID.Hex(): 1}
	sel := NewWeightedSelector(sources, weights)

	// Sequence should be [s1, s1, s1, s2] repeating
	counts := map[primitive.ObjectID]int{}
	for i := 0; i < 8; i++ {
		_, src, err := sel.NextSource(sources)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		counts[src.ID]++
	}
	if counts[s1.ID] != 6 {
		t.Fatalf("expected s1 count 6, got %d", counts[s1.ID])
	}
	if counts[s2.ID] != 2 {
		t.Fatalf("expected s2 count 2, got %d", counts[s2.ID])
	}
}

func TestWeightedSelectorDefaultWeight(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID()}
	s2 := repository.Source{ID: primitive.NewObjectID()}
	sources := []repository.Source{s1, s2}
	// Only s1 has explicit weight; s2 defaults to 1
	weights := map[string]int{s1.ID.Hex(): 2}
	sel := NewWeightedSelector(sources, weights)

	// Sequence: [s1, s1, s2] repeating
	counts := map[primitive.ObjectID]int{}
	for i := 0; i < 6; i++ {
		_, src, err := sel.NextSource(sources)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		counts[src.ID]++
	}
	if counts[s1.ID] != 4 {
		t.Fatalf("expected s1 count 4, got %d", counts[s1.ID])
	}
	if counts[s2.ID] != 2 {
		t.Fatalf("expected s2 count 2, got %d", counts[s2.ID])
	}
}

func TestWeightedSelectorDoesNotAllocateWeightSizedSequence(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID()}
	s2 := repository.Source{ID: primitive.NewObjectID()}
	sources := []repository.Source{s1, s2}
	weights := map[string]int{s1.ID.Hex(): MaxWeightedSourceWeight, s2.ID.Hex(): 1}

	sel := NewWeightedSelector(sources, weights)
	if len(sel.cumulativeWeights) != len(sources) {
		t.Fatalf("expected one cumulative weight per source, got %d", len(sel.cumulativeWeights))
	}
	if sel.totalWeight != MaxWeightedSourceWeight+1 {
		t.Fatalf("expected total weight %d, got %d", MaxWeightedSourceWeight+1, sel.totalWeight)
	}
}

func TestWeightedSelectorNoSources(t *testing.T) {
	t.Parallel()
	sel := NewWeightedSelector(nil, nil)
	_, _, err := sel.NextSource(nil)
	if err == nil {
		t.Fatal("expected error for empty sources")
	}
}

func TestUploadFileChunksWithWeightedStrategy(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	s2 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	sources := []repository.Source{s1, s2}

	clients := map[primitive.ObjectID]*stubSourceClient{}
	factory := func(_ context.Context, src *repository.Source) (sourceClient, error) {
		cli := &stubSourceClient{}
		clients[src.ID] = cli
		return cli, nil
	}

	weights := map[string]int{s1.ID.Hex(): 2, s2.ID.Hex(): 1}
	sel := NewWeightedSelector(sources, weights)

	// 9 bytes with chunk size 3 = 3 chunks
	// Weighted sequence [s1, s1, s2]: chunk0->s1, chunk1->s1, chunk2->s2
	payload := []byte("abcdefghi")
	chunks, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader(payload), sources, 3, factory, sel)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].SourceID != s1.ID || chunks[1].SourceID != s1.ID {
		t.Fatalf("first two chunks should go to s1: %+v", chunks)
	}
	if chunks[2].SourceID != s2.ID {
		t.Fatalf("third chunk should go to s2: %+v", chunks)
	}
}

func TestSelectorForBucketRoundRobin(t *testing.T) {
	t.Parallel()
	bucket := &repository.Bucket{DistributionStrategy: repository.StrategyRoundRobin}
	s1 := repository.Source{ID: primitive.NewObjectID()}
	sources := []repository.Source{s1}
	sel := SelectorForBucket(bucket, sources)
	if _, ok := sel.(*RoundRobinSelector); !ok {
		t.Fatalf("expected RoundRobinSelector, got %T", sel)
	}
}

func TestSelectorForBucketWeighted(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID()}
	bucket := &repository.Bucket{
		DistributionStrategy: repository.StrategyWeighted,
		SourceWeights:        map[string]int{s1.ID.Hex(): 5},
	}
	sources := []repository.Source{s1}
	sel := SelectorForBucket(bucket, sources)
	if _, ok := sel.(*WeightedSelector); !ok {
		t.Fatalf("expected WeightedSelector, got %T", sel)
	}
}

func TestSelectorForBucketDefault(t *testing.T) {
	t.Parallel()
	// Empty strategy should default to round-robin
	bucket := &repository.Bucket{}
	sources := []repository.Source{{ID: primitive.NewObjectID()}}
	sel := SelectorForBucket(bucket, sources)
	if _, ok := sel.(*RoundRobinSelector); !ok {
		t.Fatalf("expected RoundRobinSelector for empty strategy, got %T", sel)
	}
}

func TestUploadFileChunksFailoverToAlternateSource(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	s2 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	sources := []repository.Source{s1, s2}

	failingStub := &stubSourceClient{uploadErr: errors.New("source down")}
	workingStub := &stubSourceClient{}
	factory := func(_ context.Context, src *repository.Source) (sourceClient, error) {
		if src.ID == s1.ID {
			return failingStub, nil
		}
		return workingStub, nil
	}

	payload := []byte("abcdef")
	chunks, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader(payload), sources, 6, factory, &RoundRobinSelector{})
	if err != nil {
		t.Fatalf("expected failover to succeed, got %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	// Chunk should have been placed on s2 after s1 failed.
	if chunks[0].SourceID != s2.ID {
		t.Fatalf("expected chunk on s2 after failover, got source %s", chunks[0].SourceID.Hex())
	}
}

func TestUploadFileChunksAllSourcesFail(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	s2 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	sources := []repository.Source{s1, s2}

	factory := func(_ context.Context, src *repository.Source) (sourceClient, error) {
		return &stubSourceClient{uploadErr: errors.New("source down")}, nil
	}

	payload := []byte("abcdef")
	_, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader(payload), sources, 6, factory, &RoundRobinSelector{})
	if err == nil {
		t.Fatal("expected error when all sources fail")
	}
}

func TestUploadFileChunksSingleSourceNoFailover(t *testing.T) {
	t.Parallel()
	s1 := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	sources := []repository.Source{s1}

	factory := func(_ context.Context, src *repository.Source) (sourceClient, error) {
		return &stubSourceClient{uploadErr: errors.New("source down")}, nil
	}

	payload := []byte("abc")
	_, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader(payload), sources, 3, factory, &RoundRobinSelector{})
	if err == nil {
		t.Fatal("expected error with single source and no failover possible")
	}
}

func TestNewSourceClientNilSource(t *testing.T) {
	t.Parallel()
	_, err := NewSourceClient(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil source")
	}
}

func TestNewSourceClientUnsupportedType(t *testing.T) {
	t.Parallel()
	src := &repository.Source{Type: "unsupported_type_xyz"}
	_, err := NewSourceClient(context.Background(), src)
	if !errors.Is(err, ErrUnsupportedSourceType) {
		t.Fatalf("expected ErrUnsupportedSourceType, got %v", err)
	}
}

func TestInspectSourceUsesFactoryForGDrive(t *testing.T) {
	t.Parallel()
	src := &repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeGDrive}
	calls := 0
	factory := func(_ context.Context, got *repository.Source) (SourceClient, error) {
		calls++
		if got != src {
			t.Fatal("expected source pointer to be passed to factory")
		}
		return &stubSourceClient{
			gdriveFiles:  []gdrive.File{{ID: "file-id", Name: "file.txt", Size: 42}},
			storageTotal: 100,
			storageUsed:  42,
			storageFree:  58,
		}, nil
	}

	info, err := InspectSource(context.Background(), src, factory)
	if err != nil {
		t.Fatalf("inspect source: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected factory to be called once, got %d", calls)
	}
	if len(info.Files) != 1 || info.Files[0].ID != "file-id" || info.Files[0].Name != "file.txt" || info.Files[0].Size != 42 {
		t.Fatalf("unexpected files: %#v", info.Files)
	}
	if info.StorageTotal != 100 || info.StorageUsed != 42 || info.StorageFree != 58 {
		t.Fatalf("unexpected storage info: %#v", info)
	}
}

func TestInspectSourceUsesFactoryForS3(t *testing.T) {
	t.Parallel()
	src := &repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeS3}
	calls := 0
	factory := func(_ context.Context, got *repository.Source) (SourceClient, error) {
		calls++
		if got != src {
			t.Fatal("expected source pointer to be passed to factory")
		}
		return &stubSourceClient{
			s3Objects: []s3compat.ObjectInfo{{Key: "object-key", Size: 12}},
			s3Used:    12,
		}, nil
	}

	info, err := InspectSource(context.Background(), src, factory)
	if err != nil {
		t.Fatalf("inspect source: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected factory to be called once, got %d", calls)
	}
	if len(info.Files) != 1 || info.Files[0].ID != "object-key" || info.Files[0].Name != "object-key" || info.Files[0].Size != 12 {
		t.Fatalf("unexpected files: %#v", info.Files)
	}
	if info.StorageUsed != 12 || info.StorageTotal != 0 || info.StorageFree != 0 {
		t.Fatalf("unexpected storage info: %#v", info)
	}
}

func TestDownloadSourceFileUsesFactory(t *testing.T) {
	t.Parallel()
	src := &repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeS3}
	calls := 0
	factory := func(_ context.Context, got *repository.Source) (SourceClient, error) {
		calls++
		if got != src {
			t.Fatal("expected source pointer to be passed to factory")
		}
		return &stubSourceClient{}, nil
	}

	rc, err := DownloadSourceFile(context.Background(), src, "source-object", factory)
	if err != nil {
		t.Fatalf("download source file: %v", err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if string(data) != "source-object" {
		t.Fatalf("unexpected body %q", data)
	}
	if calls != 1 {
		t.Fatalf("expected factory to be called once, got %d", calls)
	}
}

func TestDownloadSourceFileKeepsWrappedStreamContextOpen(t *testing.T) {
	t.Parallel()
	src := &repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeS3}
	factory := func(_ context.Context, _ *repository.Source) (SourceClient, error) {
		cfg := resilience.DefaultWrapperConfig()
		cfg.MaxRetries = 0
		return resilience.Wrap(&contextAwareDownloadClient{payload: []byte("streamed-data")}, cfg), nil
	}

	rc, err := DownloadSourceFile(context.Background(), src, "source-object", factory)
	if err != nil {
		t.Fatalf("download source file: %v", err)
	}
	data, readErr := io.ReadAll(rc)
	closeErr := rc.Close()
	if readErr != nil {
		t.Fatalf("read download: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close download: %v", closeErr)
	}
	if string(data) != "streamed-data" {
		t.Fatalf("unexpected body %q", data)
	}
}

type contextAwareDownloadClient struct {
	payload []byte
}

func (c *contextAwareDownloadClient) Upload(_ context.Context, _ string, _ io.Reader) (string, error) {
	return "", nil
}

func (c *contextAwareDownloadClient) Download(ctx context.Context, _ string) (io.ReadCloser, error) {
	return &contextAwareReadCloser{ctx: ctx, reader: bytes.NewReader(c.payload)}, nil
}

func (c *contextAwareDownloadClient) Delete(_ context.Context, _ string) error {
	return nil
}

type contextAwareReadCloser struct {
	ctx    context.Context
	reader *bytes.Reader
}

func (r *contextAwareReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (r *contextAwareReadCloser) Close() error {
	return nil
}

func TestStreamFileNoChunks(t *testing.T) {
	t.Parallel()
	f := &repository.File{Chunks: []repository.FileChunk{}}
	var buf bytes.Buffer
	if err := StreamFile(context.Background(), nil, f, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty output, got %d bytes", buf.Len())
	}
}

func TestDeleteFileChunksNoChunks(t *testing.T) {
	t.Parallel()
	if err := DeleteFileChunks(context.Background(), nil, []repository.FileChunk{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteFileChunksWithFactoryReusesSourceClient(t *testing.T) {
	t.Parallel()
	srcID := primitive.NewObjectID()
	stub := &stubSourceClient{}
	factoryCalls := 0
	chunks := []repository.FileChunk{
		{SourceID: srcID, Name: "chunk0"},
		{SourceID: srcID, Name: "chunk1"},
	}

	err := deleteFileChunksWithFactory(context.Background(), chunks, func(_ context.Context, src *repository.Source) (sourceClient, error) {
		if src.ID != srcID {
			t.Fatalf("unexpected source id: %s", src.ID.Hex())
		}
		factoryCalls++
		return stub, nil
	})
	if err != nil {
		t.Fatalf("delete chunks: %v", err)
	}
	if factoryCalls != 1 {
		t.Fatalf("expected one client creation, got %d", factoryCalls)
	}
	if len(stub.deleted) != 2 || stub.deleted[0] != "chunk0" || stub.deleted[1] != "chunk1" {
		t.Fatalf("unexpected deleted chunks: %+v", stub.deleted)
	}
}

// fixedDownloadClient returns predetermined bytes for named chunks.
type fixedDownloadClient struct {
	data map[string][]byte
}

func (c *fixedDownloadClient) Upload(_ context.Context, name string, _ io.Reader) (string, error) {
	return name, nil
}

func (c *fixedDownloadClient) Download(_ context.Context, name string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(c.data[name])), nil
}

func (c *fixedDownloadClient) Delete(_ context.Context, _ string) error { return nil }

func TestUploadFileChunksStoresChecksum(t *testing.T) {
	t.Parallel()
	src := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	stub := &stubSourceClient{}
	factory := func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return stub, nil
	}

	payload := []byte("hello world")
	chunks, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader(payload), []repository.Source{src}, len(payload), factory, &RoundRobinSelector{})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	sum := sha256.Sum256(payload)
	want := hex.EncodeToString(sum[:])
	if chunks[0].Checksum != want {
		t.Fatalf("checksum mismatch: got %q, want %q", chunks[0].Checksum, want)
	}
}

func TestUploadFileChunksChecksumPerChunk(t *testing.T) {
	t.Parallel()
	src := repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}
	stub := &stubSourceClient{}
	factory := func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return stub, nil
	}

	// 6 bytes split into two 3-byte chunks
	payload := []byte("abcdef")
	chunks, err := UploadFileChunksWithStrategy(context.Background(), bytes.NewReader(payload), []repository.Source{src}, 3, factory, &RoundRobinSelector{})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	for i, part := range [][]byte{payload[:3], payload[3:]} {
		sum := sha256.Sum256(part)
		want := hex.EncodeToString(sum[:])
		if chunks[i].Checksum != want {
			t.Fatalf("chunk %d checksum: got %q, want %q", i, chunks[i].Checksum, want)
		}
	}
}

func TestStreamFileChecksumVerificationPass(t *testing.T) {
	t.Parallel()
	payload := []byte("integrity check")
	sum := sha256.Sum256(payload)
	chunk := repository.FileChunk{
		SourceID: primitive.NewObjectID(),
		Name:     "chunk0",
		Order:    0,
		Size:     int64(len(payload)),
		Checksum: hex.EncodeToString(sum[:]),
	}
	f := &repository.File{Chunks: []repository.FileChunk{chunk}}

	var buf bytes.Buffer
	err := streamFileWithFactory(context.Background(), f, &buf, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &fixedDownloadClient{data: map[string][]byte{"chunk0": payload}}, nil
	})
	if err != nil {
		t.Fatalf("expected no error for matching checksum, got %v", err)
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Fatalf("unexpected output: %q", buf.Bytes())
	}
}

func TestStreamFileChecksumVerificationFail(t *testing.T) {
	t.Parallel()
	payload := []byte("integrity check")
	sum := sha256.Sum256(payload)
	chunk := repository.FileChunk{
		SourceID: primitive.NewObjectID(),
		Name:     "chunk0",
		Order:    0,
		Size:     int64(len(payload)),
		Checksum: hex.EncodeToString(sum[:]),
	}
	f := &repository.File{Chunks: []repository.FileChunk{chunk}}

	corrupted := []byte("CORRUPTED DATA!!")
	var buf bytes.Buffer
	err := streamFileWithFactory(context.Background(), f, &buf, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &fixedDownloadClient{data: map[string][]byte{"chunk0": corrupted}}, nil
	})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}
}

func TestStreamFileChecksumRejectsOversizedChunk(t *testing.T) {
	t.Parallel()
	payload := []byte("integrity check")
	sum := sha256.Sum256(payload)
	chunk := repository.FileChunk{
		SourceID: primitive.NewObjectID(),
		Name:     "chunk0",
		Order:    0,
		Size:     int64(len(payload)),
		Checksum: hex.EncodeToString(sum[:]),
	}
	f := &repository.File{Chunks: []repository.FileChunk{chunk}}

	var buf bytes.Buffer
	err := streamFileWithFactory(context.Background(), f, &buf, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &fixedDownloadClient{data: map[string][]byte{"chunk0": append(payload, '!')}}, nil
	})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no bytes written before checksum validation, got %d", buf.Len())
	}
}

func TestStreamFileChecksumRejectsTruncatedChunk(t *testing.T) {
	t.Parallel()
	payload := []byte("integrity check")
	sum := sha256.Sum256(payload)
	chunk := repository.FileChunk{
		SourceID: primitive.NewObjectID(),
		Name:     "chunk0",
		Order:    0,
		Size:     int64(len(payload)),
		Checksum: hex.EncodeToString(sum[:]),
	}
	f := &repository.File{Chunks: []repository.FileChunk{chunk}}

	var buf bytes.Buffer
	err := streamFileWithFactory(context.Background(), f, &buf, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &fixedDownloadClient{data: map[string][]byte{"chunk0": payload[:len(payload)-1]}}, nil
	})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no bytes written before checksum validation, got %d", buf.Len())
	}
}

func TestStreamFileNoChecksumSkipsVerification(t *testing.T) {
	t.Parallel()
	payload := []byte("legacy chunk")
	// Chunk with no checksum — verification should be skipped (backwards compat).
	chunk := repository.FileChunk{
		SourceID: primitive.NewObjectID(),
		Name:     "chunk0",
		Order:    0,
		Size:     int64(len(payload)),
	}
	f := &repository.File{Chunks: []repository.FileChunk{chunk}}

	var buf bytes.Buffer
	err := streamFileWithFactory(context.Background(), f, &buf, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &fixedDownloadClient{data: map[string][]byte{"chunk0": payload}}, nil
	})
	if err != nil {
		t.Fatalf("expected no error for chunk without checksum, got %v", err)
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Fatalf("unexpected output: %q", buf.Bytes())
	}
}

func TestStreamFileRangeAcrossChunks(t *testing.T) {
	t.Parallel()
	srcID := primitive.NewObjectID()
	chunks := []repository.FileChunk{
		{SourceID: srcID, Name: "chunk0", Order: 0, Size: 5},
		{SourceID: srcID, Name: "chunk1", Order: 1, Size: 5},
		{SourceID: srcID, Name: "chunk2", Order: 2, Size: 5},
	}
	f := &repository.File{Chunks: chunks}

	var buf bytes.Buffer
	err := streamFileRangeWithFactory(context.Background(), f, &buf, 3, 11, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &fixedDownloadClient{data: map[string][]byte{
			"chunk0": []byte("abcde"),
			"chunk1": []byte("fghij"),
			"chunk2": []byte("klmno"),
		}}, nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got, want := buf.String(), "defghijkl"; got != want {
		t.Fatalf("range output mismatch: got %q, want %q", got, want)
	}
}

func TestStreamFileRangeVerifiesChecksummedChunk(t *testing.T) {
	t.Parallel()
	payload := []byte("abcdefghij")
	sum := sha256.Sum256(payload)
	chunk := repository.FileChunk{
		SourceID: primitive.NewObjectID(),
		Name:     "chunk0",
		Order:    0,
		Size:     int64(len(payload)),
		Checksum: hex.EncodeToString(sum[:]),
	}
	f := &repository.File{Chunks: []repository.FileChunk{chunk}}

	var buf bytes.Buffer
	err := streamFileRangeWithFactory(context.Background(), f, &buf, 2, 6, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &fixedDownloadClient{data: map[string][]byte{"chunk0": payload}}, nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got, want := buf.String(), "cdefg"; got != want {
		t.Fatalf("range output mismatch: got %q, want %q", got, want)
	}
}

func TestStreamFileRangeRejectsCorruptedChecksummedChunk(t *testing.T) {
	t.Parallel()
	payload := []byte("abcdefghij")
	sum := sha256.Sum256(payload)
	chunk := repository.FileChunk{
		SourceID: primitive.NewObjectID(),
		Name:     "chunk0",
		Order:    0,
		Size:     int64(len(payload)),
		Checksum: hex.EncodeToString(sum[:]),
	}
	f := &repository.File{Chunks: []repository.FileChunk{chunk}}

	var buf bytes.Buffer
	err := streamFileRangeWithFactory(context.Background(), f, &buf, 2, 6, func(_ context.Context, _ *repository.Source) (sourceClient, error) {
		return &fixedDownloadClient{data: map[string][]byte{"chunk0": []byte("abcDEFghij")}}, nil
	})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no range bytes written before checksum validation, got %d", buf.Len())
	}
}
