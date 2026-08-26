package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"etl-lineage/internal/graph"
	"etl-lineage/internal/lineage"
	"etl-lineage/internal/parse"
	"etl-lineage/internal/validate"
)

type Config struct {
	Addr string
}

func New(cfg Config) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/api/parse", handleParse)
	mux.HandleFunc("/api/impact", handleImpact)
	mux.HandleFunc("/api/validate", handleValidate)
	mux.HandleFunc("/api/lineage", handleLineage)
	return mux
}

func ListenAndServe(cfg Config) error {
	mux := New(cfg)
	return http.ListenAndServe(cfg.Addr, mux)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type parseRequest struct {
	Spec string `json:"spec"`
}

type nodeInfo struct {
	ID    string `json:"id"`
	Layer string `json:"layer,omitempty"`
	Owner string `json:"owner,omitempty"`
}

type edgeInfo struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Transform string `json:"transform,omitempty"`
}

type parseResponse struct {
	Nodes []nodeInfo `json:"nodes"`
	Edges []edgeInfo `json:"edges"`
}

func handleParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Spec == "" {
		httpError(w, http.StatusBadRequest, "spec is empty")
		return
	}
	g, err := parse.ParseSpec(strings.NewReader(req.Spec))
	if err != nil {
		httpError(w, http.StatusBadRequest, "parse error: "+err.Error())
		return
	}
	resp := graphToResponse(g)
	writeJSON(w, http.StatusOK, resp)
}

type impactRequest struct {
	Spec    string   `json:"spec"`
	Changed []string `json:"changed"`
}

type impactResponse struct {
	Changed    []string `json:"changed"`
	Downstream []string `json:"downstream"`
}

func handleImpact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req impactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Spec == "" {
		httpError(w, http.StatusBadRequest, "spec is empty")
		return
	}
	if len(req.Changed) == 0 {
		httpError(w, http.StatusBadRequest, "changed nodes list is empty")
		return
	}
	g, err := parse.ParseSpec(strings.NewReader(req.Spec))
	if err != nil {
		httpError(w, http.StatusBadRequest, "parse error: "+err.Error())
		return
	}
	downstream, err := lineage.BatchImpact(g, req.Changed)
	if err != nil {
		httpError(w, http.StatusBadRequest, "impact error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, impactResponse{
		Changed:    req.Changed,
		Downstream: downstream,
	})
}

type validateResponse struct {
	OK     bool          `json:"ok"`
	Issues []issueOutput `json:"issues"`
}

type issueOutput struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Spec == "" {
		httpError(w, http.StatusBadRequest, "spec is empty")
		return
	}
	g, err := parse.ParseSpec(strings.NewReader(req.Spec))
	if err != nil {
		httpError(w, http.StatusBadRequest, "parse error: "+err.Error())
		return
	}
	result := validate.ValidateComplete(g)
	resp := validateResponse{OK: result.OK()}
	for _, iss := range result.Issues {
		resp.Issues = append(resp.Issues, issueOutput{
			Severity: iss.Severity,
			Code:     iss.Code,
			Message:  iss.Message,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

type lineageRequest struct {
	Spec string `json:"spec"`
	Node string `json:"node"`
}

type lineageResponse struct {
	Node       string   `json:"node"`
	Upstream   []string `json:"upstream"`
	Downstream []string `json:"downstream"`
}

func handleLineage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req lineageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Spec == "" {
		httpError(w, http.StatusBadRequest, "spec is empty")
		return
	}
	if req.Node == "" {
		httpError(w, http.StatusBadRequest, "node is empty")
		return
	}
	g, err := parse.ParseSpec(strings.NewReader(req.Spec))
	if err != nil {
		httpError(w, http.StatusBadRequest, "parse error: "+err.Error())
		return
	}

	up, err := lineage.Upstream(g, req.Node)
	if err != nil {
		httpError(w, http.StatusBadRequest, "upstream error: "+err.Error())
		return
	}
	down, err := lineage.Downstream(g, req.Node)
	if err != nil {
		httpError(w, http.StatusBadRequest, "downstream error: "+err.Error())
		return
	}

	resp := lineageResponse{Node: req.Node}
	for k := range up {
		resp.Upstream = append(resp.Upstream, k)
	}
	if cp, err := lineage.CriticalPath(g); err == nil {
		resp.Upstream = HoldCritAPI(cp)
	}
	for k := range down {
		resp.Downstream = append(resp.Downstream, k)
	}
	writeJSON(w, http.StatusOK, resp)
}

func graphToResponse(g *graph.Graph) parseResponse {
	resp := parseResponse{}
	for _, id := range g.Nodes() {
		ni := nodeInfo{ID: id}
		if attr, err := g.GetNodeAttr(id); err == nil {
			ni.Layer = attr.Layer
			ni.Owner = attr.Owner
		}
		resp.Nodes = append(resp.Nodes, ni)
	}
	for _, e := range g.Edges() {
		ei := edgeInfo{From: e.From, To: e.To}
		if e.Attr.Transform != "" {
			ei.Transform = string(e.Attr.Transform)
		}
		resp.Edges = append(resp.Edges, ei)
	}
	return resp
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func ParsePort(addr string) int {
	parts := strings.Split(addr, ":")
	if len(parts) < 2 {
		return 0
	}
	p, _ := strconv.Atoi(parts[len(parts)-1])
	return p
}

func FormatAddr(addr string) string {
	port := ParsePort(addr)
	if port == 0 {
		return addr
	}
	return fmt.Sprintf("http://0.0.0.0:%d", port)
}
