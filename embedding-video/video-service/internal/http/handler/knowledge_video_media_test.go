package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"

	"video-service/internal/application/knowledgevideo"
	"video-service/internal/infrastructure/objectstorage"
)

func TestKnowledgeVideoMediaProxyUsesPersistedPrefixAndRange(t *testing.T) {
	resolver := mediaResolverStub{video: knowledgevideo.Video{ID: 8, Status: knowledgevideo.VideoReady, HLSObjectPrefix: "hls/8"}, found: true}
	store := &mediaStoreStub{body: []byte("0123456789"), info: objectstorage.ObjectInfo{Size: 10, ContentType: "video/mp2t", ETag: "etag"}}
	r := mediaRouter(resolver, store)
	req := httptest.NewRequest(http.MethodGet, "/knowledge-video-media/hls/8/segment.ts", nil)
	req.Header.Set("Range", "bytes=2-4")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusPartialContent || store.key != "hls/8/segment.ts" || w.Header().Get("ETag") != `"etag"` {
		t.Fatalf("status=%d key=%q headers=%v", w.Code, store.key, w.Header())
	}
}

func TestKnowledgeVideoMediaProxyRejectsNotReadyAndTraversal(t *testing.T) {
	store := &mediaStoreStub{}
	r := mediaRouter(mediaResolverStub{video: knowledgevideo.Video{ID: 8, Status: knowledgevideo.VideoPending}, found: true}, store)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge-video-media/hls/8/master.m3u8", nil))
	if w.Code != http.StatusNotFound || store.key != "" {
		t.Fatalf("status=%d key=%q", w.Code, store.key)
	}
}

func TestKnowledgeVideoMediaProxyRejectsIntermediateTraversal(t *testing.T) {
	store := &mediaStoreStub{info: objectstorage.ObjectInfo{Size: 1}, body: []byte("x")}
	r := mediaRouter(mediaResolverStub{video: knowledgevideo.Video{ID: 8, Status: knowledgevideo.VideoReady, HLSObjectPrefix: "hls/8"}, found: true}, store)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge-video-media/hls/8/folder/%2e%2e/master.m3u8", nil))
	if w.Code != http.StatusBadRequest || store.key != "" {
		t.Fatalf("status=%d key=%q", w.Code, store.key)
	}
}

func mediaRouter(resolver knowledgeVideoMediaResolver, store *mediaStoreStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewKnowledgeVideoMediaHandler(resolver, store)
	r.GET("/knowledge-video-media/hls/:videoId/*filepath", h.Proxy)
	return r
}

type mediaResolverStub struct {
	video knowledgevideo.Video
	found bool
}

func (s mediaResolverStub) GetVideo(context.Context, uint64) (knowledgevideo.Video, bool, error) {
	return s.video, s.found, nil
}

type mediaStoreStub struct {
	key  string
	body []byte
	info objectstorage.ObjectInfo
}

func (s *mediaStoreStub) Stat(_ context.Context, key string) (objectstorage.ObjectInfo, error) {
	s.key = key
	return s.info, nil
}
func (s *mediaStoreStub) Open(_ context.Context, key string, _ minio.GetObjectOptions) (io.ReadCloser, error) {
	s.key = key
	return io.NopCloser(bytes.NewReader(s.body)), nil
}
