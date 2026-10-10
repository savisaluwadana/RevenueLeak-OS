package server

import (
	"fmt"
	"net/http"

	"revenueleak-os/internal/core"
)

func (s *Server) scanRuns(w http.ResponseWriter, r *http.Request) {
	runs := s.Store.Snapshot().ScanRuns
	index := make([]core.ScanRunIndex, 0, len(runs))
	for _, run := range runs {
		index = append(index, core.ScanRunIndex{ID: run.ID, RuleVersion: run.RuleVersion, GraceDays: run.GraceDays, Trigger: run.Trigger, Summary: run.Summary, SourceDigest: run.SourceDigest})
	}
	respond(w, http.StatusOK, map[string]any{"runs": index, "retention_limit": core.MaxSavedScans})
}

func (s *Server) findScanRun(id string) (core.ScanRun, bool) {
	for _, run := range s.Store.Snapshot().ScanRuns {
		if run.ID == id {
			return run, true
		}
	}
	return core.ScanRun{}, false
}

func (s *Server) scanRun(w http.ResponseWriter, r *http.Request) {
	run, ok := s.findScanRun(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, "Saved run not found; it may have expired from retention")
		return
	}
	respond(w, http.StatusOK, map[string]any{"run": run})
}

func (s *Server) exportScanRun(w http.ResponseWriter, r *http.Request) {
	run, ok := s.findScanRun(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, "Saved run not found; it may have expired from retention")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "revenueleak-"+run.ID+".json"))
	respond(w, http.StatusOK, map[string]any{"schema_version": "1", "run": run})
}
