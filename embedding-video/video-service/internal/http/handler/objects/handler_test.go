package objects

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"

	"nlp-video-analysis/internal/infrastructure/objectstorage"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fakeObjectReader struct {
	info      objectstorage.ObjectInfo
	body      string
	statCalls int
	openCalls int
}

func (r *fakeObjectReader) Stat(context.Context, string) (objectstorage.ObjectInfo, error) {
	r.statCalls++
	return r.info, nil
}

func (r *fakeObjectReader) Open(context.Context, string, minio.GetObjectOptions) (io.ReadCloser, error) {
	r.openCalls++
	return io.NopCloser(strings.NewReader(r.body)), nil
}

func TestProxyVideoSetsImmutableCacheHeadersForMediaObject(t *testing.T) {
	store := &fakeObjectReader{
		info: objectstorage.ObjectInfo{Size: 5, ContentType: "video/mp2t", ETag: "abc123"},
		body: "abcde",
	}
	router := gin.New()
	router.GET("/videos/*filepath", New(store).ProxyVideo)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/videos/hls/2026/06/03/demo/v0_001.ts", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := w.Header().Get("ETag"); got != `"abc123"` {
		t.Fatalf("ETag = %q", got)
	}
	if got := w.Body.String(); got != "abcde" {
		t.Fatalf("body = %q", got)
	}
}

func TestProxyVideoReturnsNotModifiedWhenETagMatches(t *testing.T) {
	store := &fakeObjectReader{
		info: objectstorage.ObjectInfo{Size: 5, ContentType: "video/mp2t", ETag: "abc123"},
		body: "abcde",
	}
	router := gin.New()
	router.GET("/videos/*filepath", New(store).ProxyVideo)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/videos/hls/2026/06/03/demo/v0_001.ts", nil)
	req.Header.Set("If-None-Match", `"abc123"`)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d: %s", w.Code, w.Body.String())
	}
	if store.openCalls != 0 {
		t.Fatalf("expected object body not to be opened, got %d calls", store.openCalls)
	}
	if got := w.Body.String(); got != "" {
		t.Fatalf("expected empty 304 body, got %q", got)
	}
}

func TestProxyVideoCachesObjectStatForRepeatedRequests(t *testing.T) {
	store := &fakeObjectReader{
		info: objectstorage.ObjectInfo{Size: 5, ContentType: "video/mp2t", ETag: "abc123"},
		body: "abcde",
	}
	router := gin.New()
	router.GET("/videos/*filepath", New(store).ProxyVideo)

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/videos/hls/2026/06/03/demo/v0_001.ts", nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}
	if store.statCalls != 1 {
		t.Fatalf("stat calls = %d, want 1", store.statCalls)
	}
	if store.openCalls != 2 {
		t.Fatalf("open calls = %d, want 2", store.openCalls)
	}
}
