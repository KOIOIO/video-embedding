package comments_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/adminauth"
	"video-service/internal/application/videoapp"
	"video-service/internal/http/handler/comments"
	"video-service/internal/http/dto"
	"video-service/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type stubCommentApp struct {
	createCommentFunc  func(context.Context, uint64, uint64, string) (videoapp.CommentView, error)
	createReplyFunc    func(context.Context, uint64, uint64, string) (videoapp.CommentView, error)
	listCommentsFunc   func(context.Context, uint64, uint64, int, int) (videoapp.CommentListView, error)
	listRepliesFunc    func(context.Context, uint64, uint64, int, int) (videoapp.CommentListView, error)
	toggleLikeFunc     func(context.Context, uint64, uint64, videoapp.VideoReactionType) (videoapp.VideoReactionResult, error)
	getCountsFunc      func(context.Context, uint64) (int64, error)

	createCommentSegmentID uint64
	createCommentUserID    uint64
	createCommentContent   string
	createReplyCommentID   uint64
	toggleLikeCommentID    uint64
	toggleLikeUserID       uint64
}

func (s *stubCommentApp) CreateComment(ctx context.Context, segmentID uint64, userID uint64, content string) (videoapp.CommentView, error) {
	s.createCommentSegmentID = segmentID
	s.createCommentUserID = userID
	s.createCommentContent = content
	if s.createCommentFunc != nil {
		return s.createCommentFunc(ctx, segmentID, userID, content)
	}
	return videoapp.CommentView{Comment: videoapp.Comment{ID: 1, UserID: userID, VideoSegmentID: segmentID, Content: content, CreatedAt: time.Now()}, Username: "管理员"}, nil
}

func (s *stubCommentApp) CreateReply(ctx context.Context, commentID uint64, userID uint64, content string) (videoapp.CommentView, error) {
	s.createReplyCommentID = commentID
	if s.createReplyFunc != nil {
		return s.createReplyFunc(ctx, commentID, userID, content)
	}
	return videoapp.CommentView{Comment: videoapp.Comment{ID: 2, UserID: userID, ParentID: commentID, Content: content, CreatedAt: time.Now()}, Username: "管理员"}, nil
}

func (s *stubCommentApp) ListSegmentComments(ctx context.Context, segmentID uint64, viewerID uint64, page int, pageSize int) (videoapp.CommentListView, error) {
	if s.listCommentsFunc != nil {
		return s.listCommentsFunc(ctx, segmentID, viewerID, page, pageSize)
	}
	return videoapp.CommentListView{}, nil
}

func (s *stubCommentApp) ListCommentReplies(ctx context.Context, commentID uint64, viewerID uint64, page int, pageSize int) (videoapp.CommentListView, error) {
	if s.listRepliesFunc != nil {
		return s.listRepliesFunc(ctx, commentID, viewerID, page, pageSize)
	}
	return videoapp.CommentListView{}, nil
}

func (s *stubCommentApp) ToggleCommentLike(ctx context.Context, commentID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, error) {
	s.toggleLikeCommentID = commentID
	s.toggleLikeUserID = userID
	if s.toggleLikeFunc != nil {
		return s.toggleLikeFunc(ctx, commentID, userID, reactionType)
	}
	return videoapp.VideoReactionResult{Active: true, ReactionType: reactionType, Counts: videoapp.VideoReactionCounts{LikeCount: 1}}, nil
}

func (s *stubCommentApp) GetSegmentCommentCount(ctx context.Context, segmentID uint64) (int64, error) {
	if s.getCountsFunc != nil {
		return s.getCountsFunc(ctx, segmentID)
	}
	return 0, nil
}

func newTestRouter(app any) *gin.Engine {
	router := gin.New()
	h := comments.New(app)
	router.GET("/api/video-segments/:id/comments", h.ListComments)
	router.POST("/api/video-segments/:id/comments", h.CreateComment)
	router.GET("/api/video-segments/:id/comment-counts", h.GetCommentCounts)
	router.GET("/api/comments/:id/replies", h.ListReplies)
	router.POST("/api/comments/:id/replies", h.CreateReply)
	router.POST("/api/comments/:id/reactions", h.ToggleCommentLike)
	return router
}

func setAdmin(c *gin.Context, userID uint64) {
	c.Set(middleware.AdminContextKey, adminauth.Admin{ID: userID})
}

func decodeData[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var response dto.SuccessResponse[T]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response.Data
}

func TestListCommentsPassesViewerAndPagination(t *testing.T) {
	app := &stubCommentApp{
		listCommentsFunc: func(_ context.Context, segmentID uint64, viewerID uint64, page int, pageSize int) (videoapp.CommentListView, error) {
			if segmentID != 21 || viewerID != 7 || page != 2 || pageSize != 5 {
				t.Fatalf("unexpected args: segment=%d viewer=%d page=%d size=%d", segmentID, viewerID, page, pageSize)
			}
			return videoapp.CommentListView{
				Total: 3,
				Comments: []videoapp.CommentView{{
					Comment:          videoapp.Comment{ID: 1, UserID: 1, Content: "好", LikeCount: 2, CreatedAt: time.Unix(1700000000, 0)},
					Username:         "管理员",
					UserReactionType: videoapp.VideoReactionLike,
				}},
			}, nil
		},
	}
	router := newTestRouter(app)
	req := httptest.NewRequest(http.MethodGet, "/api/video-segments/21/comments?user_id=7&page=2&page_size=5", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	data := decodeData[dto.CommentListData](t, recorder)
	if data.Total != 3 || len(data.Comments) != 1 {
		t.Fatalf("unexpected data: %+v", data)
	}
	comment := data.Comments[0]
	if comment.Username != "管理员" || comment.UserReactionType != "like" || comment.CreatedAtUnix != 1700000000 {
		t.Fatalf("unexpected comment: %+v", comment)
	}
}

func TestListCommentsRejectsBadParams(t *testing.T) {
	router := newTestRouter(&stubCommentApp{})
	for _, path := range []string{
		"/api/video-segments/bad/comments",
		"/api/video-segments/1/comments?user_id=bad",
		"/api/video-segments/1/comments?page=bad",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", path, recorder.Code)
		}
	}
}

func TestCreateCommentUsesAuthenticatedUser(t *testing.T) {
	app := &stubCommentApp{}
	router := newTestRouter(app)
	req := httptest.NewRequest(http.MethodPost, "/api/video-segments/21/comments", bytes.NewBufferString(`{"content":"  讲得好  "}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unauthenticated status = %d, want 400", recorder.Code)
	}

	router = gin.New()
	h := comments.New(app)
	router.POST("/api/video-segments/:id/comments", func(c *gin.Context) {
		setAdmin(c, 7)
		h.CreateComment(c)
	})
	req = httptest.NewRequest(http.MethodPost, "/api/video-segments/21/comments", bytes.NewBufferString(`{"content":"  讲得好  "}`))
	req.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if app.createCommentSegmentID != 21 || app.createCommentUserID != 7 || app.createCommentContent != "  讲得好  " {
		t.Fatalf("unexpected create args: %+v", app)
	}
}

func TestToggleCommentLikeUsesAuthenticatedUser(t *testing.T) {
	app := &stubCommentApp{}
	router := gin.New()
	h := comments.New(app)
	router.POST("/api/comments/:id/reactions", func(c *gin.Context) {
		setAdmin(c, 7)
		h.ToggleCommentLike(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/comments/31/reactions", bytes.NewBufferString(`{"reaction_type":"double_like"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if app.toggleLikeCommentID != 31 || app.toggleLikeUserID != 7 {
		t.Fatalf("unexpected toggle args: comment=%d user=%d", app.toggleLikeCommentID, app.toggleLikeUserID)
	}
	data := decodeData[dto.CommentLikeData](t, recorder)
	if !data.Active || data.ReactionType != "double_like" || data.LikeCount != 1 {
		t.Fatalf("unexpected like data: %+v", data)
	}
}

func TestToggleCommentLikeRejectsBadReactionType(t *testing.T) {
	app := &stubCommentApp{}
	router := gin.New()
	h := comments.New(app)
	router.POST("/api/comments/:id/reactions", func(c *gin.Context) {
		setAdmin(c, 7)
		h.ToggleCommentLike(c)
	})
	for _, body := range []string{`{"reaction_type":"bad"}`, `{}`, `not json`} {
		req := httptest.NewRequest(http.MethodPost, "/api/comments/31/reactions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want 400", body, recorder.Code)
		}
	}
}

func TestCommentNotFoundMapsTo404(t *testing.T) {
	app := &stubCommentApp{
		createReplyFunc: func(context.Context, uint64, uint64, string) (videoapp.CommentView, error) {
			return videoapp.CommentView{}, videoapp.ErrCommentNotFound
		},
		toggleLikeFunc: func(context.Context, uint64, uint64, videoapp.VideoReactionType) (videoapp.VideoReactionResult, error) {
			return videoapp.VideoReactionResult{}, videoapp.ErrCommentNotFound
		},
		listRepliesFunc: func(context.Context, uint64, uint64, int, int) (videoapp.CommentListView, error) {
			return videoapp.CommentListView{}, videoapp.ErrCommentNotFound
		},
	}
	router := gin.New()
	h := comments.New(app)
	router.POST("/api/comments/:id/replies", func(c *gin.Context) {
		setAdmin(c, 7)
		h.CreateReply(c)
	})
	router.POST("/api/comments/:id/reactions", func(c *gin.Context) {
		setAdmin(c, 7)
		h.ToggleCommentLike(c)
	})
	router.GET("/api/comments/:id/replies", h.ListReplies)

	req := httptest.NewRequest(http.MethodPost, "/api/comments/31/replies", bytes.NewBufferString(`{"content":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("reply status = %d, want 404", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/comments/31/reactions", bytes.NewBufferString(`{"reaction_type":"like"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("reaction status = %d, want 404", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/comments/31/replies", nil)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("replies status = %d, want 404", recorder.Code)
	}
}

func TestGetCommentCounts(t *testing.T) {
	app := &stubCommentApp{
		getCountsFunc: func(_ context.Context, segmentID uint64) (int64, error) {
			if segmentID != 21 {
				t.Fatalf("segment = %d, want 21", segmentID)
			}
			return 42, nil
		},
	}
	router := newTestRouter(app)
	req := httptest.NewRequest(http.MethodGet, "/api/video-segments/21/comment-counts", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	data := decodeData[dto.SegmentCommentCountData](t, recorder)
	if data.VideoSegmentID != 21 || data.Total != 42 {
		t.Fatalf("unexpected counts data: %+v", data)
	}
}

func TestUnexpectedErrorMapsTo500(t *testing.T) {
	app := &stubCommentApp{
		listCommentsFunc: func(context.Context, uint64, uint64, int, int) (videoapp.CommentListView, error) {
			return videoapp.CommentListView{}, errors.New("boom")
		},
	}
	router := newTestRouter(app)
	req := httptest.NewRequest(http.MethodGet, "/api/video-segments/21/comments", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
}
