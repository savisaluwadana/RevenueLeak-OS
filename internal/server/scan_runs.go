package server

import (
	"fmt"
	"log"
	"net/http"

	"revenueleak-os/internal/core"
)

func (s *Server) scanRuns(w http.ResponseWriter, r *http.Request) {
	d, ok := s.readWorkspace(w)
	if !ok {
		return
	}
	index := make([]core.ScanRunIndex, 0, len(d.ScanRuns))
	for _, run := range d.ScanRuns {
		index = append(index, core.ScanRunIndex{ID: run.ID, RuleVersion: run.RuleVersion, GraceDays: run.GraceDays, Trigger: run.Trigger, Summary: run.Summary, SourceDigest: run.SourceDigest})
	}
	respond(w, http.StatusOK, map[string]any{"runs": index, "retention_limit": core.MaxSavedScans})
}

func (s *Server) findScanRun(id string) (core.ScanRun, bool, error) {
	d, err := s.Store.Read()
	if err != nil {
		return core.ScanRun{}, false, err
	}
	for _, run := range d.ScanRuns {
		if run.ID == id {
			return run, true, nil
		}
	}
	return core.ScanRun{}, false, nil
}

func (s *Server) scanRun(w http.ResponseWriter, r *http.Request) {
	run, ok, err := s.findScanRun(r.PathValue("id"))
	if err != nil {
		log.Printf("scan evidence read failed: %v", err)
		fail(w, http.StatusInternalServerError, "Unable to load workspace data")
		return
	}
	if !ok {
		fail(w, http.StatusNotFound, "Saved run not found; it may have expired from retention")
		return
	}
	respond(w, http.StatusOK, map[string]any{"run": run})
}

func (s *Server) exportScanRun(w http.ResponseWriter, r *http.Request) {
	run, ok, err := s.findScanRun(r.PathValue("id"))
	if err != nil {
		log.Printf("scan evidence export read failed: %v", err)
		fail(w, http.StatusInternalServerError, "Unable to load workspace data")
		return
	}
	if !ok {
		fail(w, http.StatusNotFound, "Saved run not found; it may have expired from retention")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "revenueleak-"+run.ID+".json"))
	respond(w, http.StatusOK, map[string]any{"schema_version": "1", "run": run})
}
