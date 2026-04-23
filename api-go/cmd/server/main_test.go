package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

type fakeGracefulServer struct {
	listenErr      error
	shutdownErr    error
	listenStarted  chan struct{}
	releaseListen  chan struct{}
	shutdownCalled chan context.Context
}

func newFakeGracefulServer(listenErr error) *fakeGracefulServer {
	return &fakeGracefulServer{
		listenErr:      listenErr,
		listenStarted:  make(chan struct{}),
		releaseListen:  make(chan struct{}),
		shutdownCalled: make(chan context.Context, 1),
	}
}

func (s *fakeGracefulServer) ListenAndServe() error {
	close(s.listenStarted)
	<-s.releaseListen
	return s.listenErr
}

func (s *fakeGracefulServer) Shutdown(ctx context.Context) error {
	s.shutdownCalled <- ctx
	close(s.releaseListen)
	return s.shutdownErr
}

type immediateServer struct {
	err error
}

func (s immediateServer) ListenAndServe() error {
	return s.err
}

func (s immediateServer) Shutdown(context.Context) error {
	return nil
}

func TestRunServerGracefulShutdown(t *testing.T) {
	server := newFakeGracefulServer(http.ErrServerClosed)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- runServer(ctx, server, time.Second)
	}()

	<-server.listenStarted
	cancel()

	shutdownCtx := <-server.shutdownCalled
	if _, ok := shutdownCtx.Deadline(); !ok {
		t.Fatal("shutdown context has no deadline")
	}

	if err := <-done; err != nil {
		t.Fatalf("runServer returned error: %v", err)
	}
}

func TestRunServerReturnsListenError(t *testing.T) {
	wantErr := errors.New("listen failed")
	err := runServer(context.Background(), immediateServer{err: wantErr}, time.Second)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runServer error = %v, want %v", err, wantErr)
	}
}

func TestRunServerReturnsShutdownError(t *testing.T) {
	wantErr := errors.New("shutdown failed")
	server := newFakeGracefulServer(http.ErrServerClosed)
	server.shutdownErr = wantErr
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- runServer(ctx, server, time.Second)
	}()

	<-server.listenStarted
	cancel()

	err := <-done
	if !errors.Is(err, wantErr) {
		t.Fatalf("runServer error = %v, want %v", err, wantErr)
	}
}
