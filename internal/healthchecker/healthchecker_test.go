package healthchecker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDownloadBlobRangeRequestsFourMegabytes(t *testing.T) {
	const limit = int64(4 * 1024 * 1024)

	var gotRange string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(make([]byte, limit))
	}))
	defer server.Close()

	downloaded, elapsedMs, err := downloadBlobRange(context.Background(), server.Client(), server.URL, "", limit)
	if err != nil {
		t.Fatalf("downloadBlobRange() error = %v", err)
	}
	if gotRange != "bytes=0-4194303" {
		t.Fatalf("Range = %q, want %q", gotRange, "bytes=0-4194303")
	}
	if downloaded != limit {
		t.Fatalf("downloaded = %d, want %d", downloaded, limit)
	}
	if elapsedMs < 0 {
		t.Fatalf("elapsedMs = %d, want >= 0", elapsedMs)
	}
}

func TestDownloadBlobRangeHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, _, err := downloadBlobRange(ctx, server.Client(), server.URL, "", 4*1024*1024)
	if err == nil {
		t.Fatal("downloadBlobRange() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("downloadBlobRange() error = %v, want context deadline exceeded", err)
	}
}

func TestSpeedTestStageContextDoesNotConsumeNextStageBudget(t *testing.T) {
	parent := context.Background()

	stage1, cancel1 := speedTestStageContext(parent, 20*time.Millisecond)
	defer cancel1()
	<-stage1.Done()
	if !errors.Is(stage1.Err(), context.DeadlineExceeded) {
		t.Fatalf("stage1 err = %v, want context deadline exceeded", stage1.Err())
	}

	stage2, cancel2 := speedTestStageContext(parent, 50*time.Millisecond)
	defer cancel2()

	select {
	case <-stage2.Done():
		t.Fatalf("stage2 was already canceled: %v", stage2.Err())
	case <-time.After(10 * time.Millisecond):
	}
}
