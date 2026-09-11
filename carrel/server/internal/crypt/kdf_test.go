package crypt

import (
	"context"
	"testing"
	"time"
)

func TestKDFSemaphoreFifthWaitsForSlot(t *testing.T) {
	for range kdfConcurrency {
		if err := acquireKDF(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan struct{})
	go func() {
		_, err := DeriveKey(context.Background(), "pass", []byte("salt123456789012"), LoginKDF)
		if err != nil {
			t.Error(err)
		}
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("KDF ran while semaphore full")
	case <-time.After(50 * time.Millisecond):
	}
	releaseKDF()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("KDF did not run after slot opened")
	}
	for range kdfConcurrency - 1 {
		releaseKDF()
	}
}

func TestKDFSemaphoreWaitsOnContext(t *testing.T) {
	for range kdfConcurrency {
		if err := acquireKDF(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := DeriveKey(ctx, "pass", []byte("salt123456789012"), LoginKDF)
	if err != context.DeadlineExceeded {
		t.Fatalf("got %v, want context.DeadlineExceeded", err)
	}
	for range kdfConcurrency {
		releaseKDF()
	}
}
