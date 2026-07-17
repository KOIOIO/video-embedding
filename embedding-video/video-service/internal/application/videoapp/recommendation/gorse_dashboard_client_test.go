package recommendation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestGorseDashboardClientTimeseriesLogsInAndNormalizesPoints(t *testing.T) {
	loginCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse login form: %v", err)
			}
			if r.Form.Get("user_name") != "admin" || r.Form.Get("password") != "secret" {
				t.Fatalf("unexpected credentials %q/%q", r.Form.Get("user_name"), r.Form.Get("password"))
			}
			loginCount++
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "valid", Path: "/"})
			w.WriteHeader(http.StatusNoContent)
		case "/api/dashboard/timeseries/positive_feedback_ratio":
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "valid" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.URL.Query().Get("begin") != "2026-07-09T00:00:00Z" || r.URL.Query().Get("end") != "2026-07-16T23:59:59Z" {
				t.Fatalf("unexpected range: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"Name": "positive_feedback_ratio", "Timestamp": "2026-07-10T00:00:00Z", "Value": 0.14285714285714285},
				{"Name": "positive_feedback_ratio", "Timestamp": "2026-07-14T00:00:00Z", "Value": 0.375},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGorseDashboardHTTPClient(GorseDashboardClientConfig{
		Endpoint: server.URL,
		Username: "admin",
		Password: "secret",
		Timeout:  time.Second,
	})
	begin := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 16, 23, 59, 59, 0, time.UTC)

	points, err := client.Timeseries(context.Background(), "positive_feedback_ratio", begin, end)
	if err != nil {
		t.Fatalf("Timeseries returned error: %v", err)
	}
	if loginCount != 1 {
		t.Fatalf("login count = %d, want 1", loginCount)
	}
	want := []GorseDashboardPoint{
		{Timestamp: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), Value: 0.14285714285714285},
		{Timestamp: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC), Value: 0.375},
	}
	if !reflect.DeepEqual(points, want) {
		t.Fatalf("points = %#v, want %#v", points, want)
	}
}

func TestGorseDashboardClientTimeseriesReauthenticatesOnceAfterUnauthorized(t *testing.T) {
	loginCount := 0
	activeSession := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginCount++
			activeSession = "session-" + strconv.Itoa(loginCount)
			http.SetCookie(w, &http.Cookie{Name: "session", Value: activeSession, Path: "/"})
			w.WriteHeader(http.StatusNoContent)
		case "/api/dashboard/timeseries/cf_ndcg":
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != activeSession || activeSession == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"Name": "cf_ndcg", "Timestamp": "2026-07-14T00:00:00Z", "Value": 0.61,
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGorseDashboardHTTPClient(GorseDashboardClientConfig{
		Endpoint: server.URL,
		Username: "admin",
		Password: "secret",
		Timeout:  time.Second,
	})
	begin := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 16, 23, 59, 59, 0, time.UTC)
	if _, err := client.Timeseries(context.Background(), "cf_ndcg", begin, end); err != nil {
		t.Fatalf("first Timeseries returned error: %v", err)
	}

	activeSession = "expired"
	if _, err := client.Timeseries(context.Background(), "cf_ndcg", begin, end); err != nil {
		t.Fatalf("second Timeseries returned error: %v", err)
	}
	if loginCount != 2 {
		t.Fatalf("login count = %d, want 2", loginCount)
	}
}

func TestGorseDashboardClientPositiveFeedbackTypesUsesDashboardConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "valid", Path: "/"})
			w.WriteHeader(http.StatusNoContent)
		case "/api/dashboard/config":
			if _, err := r.Cookie("session"); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"recommend":{"data_source":{"positive_feedback_types":["like","double_like","watch>=0.6"]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGorseDashboardHTTPClient(GorseDashboardClientConfig{
		Endpoint: server.URL, Username: "admin", Password: "secret", Timeout: time.Second,
	})
	types, err := client.PositiveFeedbackTypes(context.Background())
	if err != nil {
		t.Fatalf("PositiveFeedbackTypes returned error: %v", err)
	}
	if !reflect.DeepEqual(types, []string{"like", "double_like", "watch>=0.6"}) {
		t.Fatalf("types = %#v", types)
	}
}

func TestGorseDashboardClientRejectsUnexpectedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "valid", Path: "/"})
			w.WriteHeader(http.StatusNoContent)
		case "/api/dashboard/timeseries/cf_precision":
			if _, err := r.Cookie("session"); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`[{"Name":"cf_precision","Timestamp":"not-a-time","Value":0.5}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGorseDashboardHTTPClient(GorseDashboardClientConfig{
		Endpoint: server.URL, Username: "admin", Password: "secret", Timeout: time.Second,
	})
	_, err := client.Timeseries(context.Background(), "cf_precision", time.Now().Add(-time.Hour), time.Now())
	if err == nil {
		t.Fatal("expected invalid timestamp error")
	}
}
