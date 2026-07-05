package recommendation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestGorseHTTPClientRecommendSendsAPIKeyAndParsesSegmentIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/recommend/7" {
			t.Fatalf("path = %q, want /api/recommend/7", r.URL.Path)
		}
		if r.URL.Query().Get("n") != "5" {
			t.Fatalf("n = %q, want 5", r.URL.Query().Get("n"))
		}
		if got := r.Header.Get("X-API-Key"); got != "secret" {
			t.Fatalf("X-API-Key = %q, want secret", got)
		}
		_ = json.NewEncoder(w).Encode([]string{"101", "bad", "0", "202"})
	}))
	defer server.Close()

	client := NewGorseHTTPClient(GorseClientConfig{
		Endpoint: server.URL,
		APIKey:   "secret",
		Timeout:  time.Second,
	})

	got, err := client.Recommend(context.Background(), 7, 5)
	if err != nil {
		t.Fatalf("Recommend returned error: %v", err)
	}
	want := []uint64{101, 202}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Recommend() = %#v, want %#v", got, want)
	}
}

func TestGorseHTTPClientRecommendReturnsErrorOnNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewGorseHTTPClient(GorseClientConfig{Endpoint: server.URL, Timeout: time.Second})
	if _, err := client.Recommend(context.Background(), 7, 5); err == nil {
		t.Fatal("expected error")
	}
}

func TestGorseHTTPClientPutFeedbackUsesFeedbackEndpoint(t *testing.T) {
	var payload []GorseFeedback
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api/feedback" {
			t.Fatalf("path = %q, want /api/feedback", r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "secret" {
			t.Fatalf("X-API-Key = %q, want secret", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewGorseHTTPClient(GorseClientConfig{Endpoint: server.URL, APIKey: "secret", Timeout: time.Second})
	now := time.Date(2026, 6, 26, 10, 30, 0, 0, time.UTC)
	err := client.PutFeedback(context.Background(), []GorseFeedback{{
		FeedbackType: "like",
		UserID:       "7",
		ItemID:       "101",
		Timestamp:    now,
		Value:        2,
	}})
	if err != nil {
		t.Fatalf("PutFeedback returned error: %v", err)
	}
	if len(payload) != 1 || payload[0].FeedbackType != "like" || payload[0].UserID != "7" || payload[0].ItemID != "101" || payload[0].Value != 2 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestGorseHTTPClientBatchUpsertsUsersAndItems(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/users":
			var payload []GorseUser
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode users: %v", err)
			}
			if len(payload) != 1 || payload[0].UserID != "7" {
				t.Fatalf("users payload = %+v", payload)
			}
		case "/api/items":
			var payload []GorseItem
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode items: %v", err)
			}
			if len(payload) != 1 || payload[0].ItemID != "101" {
				t.Fatalf("items payload = %+v", payload)
			}
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewGorseHTTPClient(GorseClientConfig{Endpoint: server.URL, Timeout: time.Second})
	if err := client.UpsertUsers(context.Background(), []GorseUser{{UserID: "7"}}); err != nil {
		t.Fatalf("UpsertUsers returned error: %v", err)
	}
	if err := client.UpsertItems(context.Background(), []GorseItem{{ItemID: "101"}}); err != nil {
		t.Fatalf("UpsertItems returned error: %v", err)
	}
	want := []string{"POST /api/users", "POST /api/items"}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("seen = %#v, want %#v", seen, want)
	}
}

func TestGorseHTTPClientUpsertItemsSendsExplicitHiddenFalse(t *testing.T) {
	var raw map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode items: %v", err)
		}
		if len(payload) != 1 {
			t.Fatalf("payload len = %d, want 1", len(payload))
		}
		raw = payload[0]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewGorseHTTPClient(GorseClientConfig{Endpoint: server.URL, Timeout: time.Second})
	if err := client.UpsertItems(context.Background(), []GorseItem{{ItemID: "101", IsHidden: false}}); err != nil {
		t.Fatalf("UpsertItems returned error: %v", err)
	}

	value, ok := raw["IsHidden"]
	if !ok {
		t.Fatal("IsHidden missing from payload")
	}
	if value != false {
		t.Fatalf("IsHidden = %#v, want false", value)
	}
}

func TestGorseHTTPClientPatchItemUsesItemEndpoint(t *testing.T) {
	var raw map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/api/item/101" {
			t.Fatalf("path = %q, want /api/item/101", r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "secret" {
			t.Fatalf("X-API-Key = %q, want secret", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode item: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewGorseHTTPClient(GorseClientConfig{Endpoint: server.URL, APIKey: "secret", Timeout: time.Second})
	if err := client.PatchItem(context.Background(), GorseItem{ItemID: "101", IsHidden: false}); err != nil {
		t.Fatalf("PatchItem returned error: %v", err)
	}

	if raw["ItemId"] != "101" {
		t.Fatalf("ItemId = %#v, want 101", raw["ItemId"])
	}
	if raw["IsHidden"] != false {
		t.Fatalf("IsHidden = %#v, want false", raw["IsHidden"])
	}
}
