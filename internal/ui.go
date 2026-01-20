package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/luccadibe/benchctl/internal/uiassets"
)

type runSummary struct {
	ID            string            `json:"id"`
	BenchmarkName string            `json:"benchmark_name"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	Custom        map[string]string `json:"custom,omitempty"`
}

type fileInfo struct {
	Name  string    `json:"name"`
	Ext   string    `json:"ext"`
	Size  int64     `json:"size"`
	Mtime time.Time `json:"mtime"`
	Kind  string    `json:"kind"`
}

type runDetailResponse struct {
	Metadata *RunMetadata `json:"metadata"`
	Files    []fileInfo   `json:"files"`
}

type uiServer struct {
	outputDir string
	assets    fs.FS
}

func ServeUI(addr, outputDir string) error {
	if strings.TrimSpace(outputDir) == "" {
		return errors.New("output directory is required")
	}

	assets, err := fs.Sub(uiassets.Dist, "dist")
	if err != nil {
		return fmt.Errorf("failed to load embedded UI assets: %w", err)
	}

	server := &uiServer{
		outputDir: outputDir,
		assets:    assets,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/runs", server.handleRuns)
	mux.HandleFunc("/api/runs/", server.handleRun)
	mux.Handle("/", http.HandlerFunc(server.serveStatic))

	return http.ListenAndServe(addr, mux)
}

func (s *uiServer) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	runs, err := s.listRuns()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, runs)
}

func (s *uiServer) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	runID := parts[0]
	if runID == "" {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 1 {
		detail, err := s.runDetail(runID)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, detail)
		return
	}

	if len(parts) >= 3 && parts[1] == "files" {
		name, err := url.PathUnescape(strings.Join(parts[2:], "/"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid file name")
			return
		}
		if name == "" || name != filepath.Base(name) || strings.Contains(name, string(os.PathSeparator)) {
			writeJSONError(w, http.StatusBadRequest, "invalid file name")
			return
		}
		if name == "metadata.json" {
			http.NotFound(w, r)
			return
		}
		if err := s.serveRunFile(w, r, runID, name); err != nil {
			writeJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		return
	}

	http.NotFound(w, r)
}

func (s *uiServer) listRuns() ([]runSummary, error) {
	entries, err := os.ReadDir(s.outputDir)
	if err != nil {
		return nil, err
	}
	runs := make([]runSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		md, err := LoadRunMetadata(filepath.Join(s.outputDir, runID, "metadata.json"))
		if err != nil {
			continue
		}
		runs = append(runs, runSummary{
			ID:            runID,
			BenchmarkName: md.BenchmarkName,
			StartTime:     md.StartTime,
			EndTime:       md.EndTime,
			Custom:        md.Custom,
		})
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartTime.After(runs[j].StartTime)
	})
	return runs, nil
}

func (s *uiServer) runDetail(runID string) (*runDetailResponse, error) {
	runDir := filepath.Join(s.outputDir, runID)
	metadata, err := LoadRunMetadata(filepath.Join(runDir, "metadata.json"))
	if err != nil {
		return nil, err
	}
	files, err := listRunFiles(runDir)
	if err != nil {
		return nil, err
	}
	return &runDetailResponse{
		Metadata: metadata,
		Files:    files,
	}, nil
}

func (s *uiServer) serveRunFile(w http.ResponseWriter, r *http.Request, runID, name string) error {
	runDir := filepath.Join(s.outputDir, runID)
	path := filepath.Join(runDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return fmt.Errorf("file not found")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	} else if ct := mime.TypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeFile(w, r, path)
	return nil
}

func listRunFiles(runDir string) ([]fileInfo, error) {
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return nil, err
	}
	files := make([]fileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "metadata.json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		files = append(files, fileInfo{
			Name:  name,
			Ext:   strings.TrimPrefix(ext, "."),
			Size:  info.Size(),
			Mtime: info.ModTime(),
			Kind:  classifyFile(ext),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	return files, nil
}

func classifyFile(ext string) string {
	switch strings.ToLower(ext) {
	case ".csv":
		return "csv"
	case ".png", ".jpg", ".jpeg":
		return "image"
	default:
		return "other"
	}
}

func (s *uiServer) serveStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	requestPath := path.Clean(r.URL.Path)
	requestPath = strings.TrimPrefix(requestPath, "/")
	if requestPath == "" || requestPath == "." {
		s.serveIndex(w)
		return
	}
	if path.Ext(requestPath) != "" {
		if s.serveGzipAsset(w, requestPath) {
			return
		}
		http.NotFound(w, r)
		return
	}
	if s.serveGzipAsset(w, requestPath) {
		return
	}
	s.serveIndex(w)
}

func (s *uiServer) serveIndex(w http.ResponseWriter) {
	data, err := fs.ReadFile(s.assets, "index.html.gz")
	if err != nil {
		http.Error(w, "ui index not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *uiServer) serveGzipAsset(w http.ResponseWriter, requestPath string) bool {
	gzipPath := requestPath + ".gz"
	info, err := fs.Stat(s.assets, gzipPath)
	if err != nil || info.IsDir() {
		return false
	}
	data, err := fs.ReadFile(s.assets, gzipPath)
	if err != nil {
		return false
	}
	if contentType := mime.TypeByExtension(path.Ext(requestPath)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Encoding", "gzip")
	_, _ = w.Write(data)
	return true
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
