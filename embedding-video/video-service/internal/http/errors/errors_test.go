package errors_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	httperrors "nlp-video-analysis/internal/http/errors"
)

func TestWriteError_InvalidArgument(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	httperrors.Write(c, httperrors.InvalidArgument("question_text is required"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":"invalid_argument"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"message":"question_text is required"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"success":false`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestWriteError_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":"video_not_found"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestWriteError_NilFallsBackToInternal(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	httperrors.Write(c, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":"internal"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestWriteError_ManualAPIErrorUsesDefaults(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	httperrors.Write(c, &httperrors.APIError{})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":"internal"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"message":"internal server error"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestWriteError_AbortsContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	httperrors.Write(c, httperrors.InvalidArgument("question_text is required"))

	if !c.IsAborted() {
		t.Fatal("expected context to be aborted")
	}
}
