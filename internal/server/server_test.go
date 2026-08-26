package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleSpec = `transform_clean <- source_db
staging <- transform_clean
aggregate <- staging
report_table <- aggregate
`

func TestHealthEndpoint(t *testing.T) {
	mux := New(Config{Addr: ":8080"})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestParseEndpoint(t *testing.T) {
	mux := New(Config{Addr: ":8080"})
	payload := parseRequest{Spec: sampleSpec}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/parse", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp parseResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(resp.Nodes))
	}
	if len(resp.Edges) != 4 {
		t.Errorf("expected 4 edges, got %d", len(resp.Edges))
	}
}

func TestParseEndpoint_EmptySpec(t *testing.T) {
	mux := New(Config{Addr: ":8080"})
	body := []byte(`{"spec":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/parse", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestImpactEndpoint(t *testing.T) {
	mux := New(Config{Addr: ":8080"})
	payload := impactRequest{Spec: sampleSpec, Changed: []string{"source_db"}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/impact", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp impactResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Downstream) == 0 {
		t.Error("expected downstream nodes")
	}
}

func TestImpactEndpoint_EmptyChanged(t *testing.T) {
	mux := New(Config{Addr: ":8080"})
	body := []byte(`{"spec":"a -> b","changed":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/impact", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestValidateEndpoint(t *testing.T) {
	mux := New(Config{Addr: ":8080"})
	payload := parseRequest{Spec: sampleSpec}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp validateResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.OK {
		t.Errorf("expected OK=true, got false")
	}
}

func TestLineageEndpoint(t *testing.T) {
	mux := New(Config{Addr: ":8080"})
	payload := lineageRequest{Spec: sampleSpec, Node: "staging"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/lineage", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp lineageResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Node != "staging" {
		t.Errorf("expected node=staging, got %q", resp.Node)
	}
	if len(resp.Upstream) == 0 {
		t.Error("expected upstream nodes")
	}
	if len(resp.Downstream) == 0 {
		t.Error("expected downstream nodes")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	mux := New(Config{Addr: ":8080"})
	endpoints := []string{"/api/parse", "/api/impact", "/api/validate", "/api/lineage"}
	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", ep, rec.Code)
		}
	}
}

func TestParsePort(t *testing.T) {
	if p := ParsePort(":8080"); p != 8080 {
		t.Errorf("expected 8080, got %d", p)
	}
}
