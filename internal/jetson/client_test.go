package jetson

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/config"
)

func TestFetchForDateUsesHistoryEndpointAndAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", request.Method)
		}
		if request.URL.Path != "/health-status/2026-09-01" {
			t.Fatalf("unexpected request path %q", request.URL.Path)
		}
		if request.Header.Get("x-api-key") != "secret" {
			t.Fatalf("missing API key header")
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Fatalf("unexpected Accept header %q", request.Header.Get("Accept"))
		}
		_, _ = w.Write([]byte(`[{"cow_id":3}]`))
	}))
	defer server.Close()

	client := NewClient(config.Config{
		JetsonBaseURL:    server.URL,
		JetsonAPIKey:     "secret",
		JetsonAPIKeyName: "x-api-key",
		RequestTimeout:   time.Second,
	})
	payloads, err := client.FetchForDate(context.Background(), time.Date(2026, 9, 1, 23, 0, 0, 0, time.FixedZone("UTC+1", 3600)))
	if err != nil {
		t.Fatalf("FetchForDate returned error: %v", err)
	}
	if len(payloads) != 1 || payloads[0]["cow_id"] != float64(3) {
		t.Fatalf("unexpected payloads %#v", payloads)
	}
}

func TestFetchForDateReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(config.Config{JetsonBaseURL: server.URL, RequestTimeout: time.Second})
	_, err := client.FetchForDate(context.Background(), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected HTTP error")
	}
}
