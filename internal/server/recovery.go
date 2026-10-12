package server

import (
	"errors"
	"log"
	"net/http"
	"time"

	"revenueleak-os/internal/core"
)

func (s *Server) recoveries(w http.ResponseWriter, r *http.Request) {
	d, ok := s.readWorkspace(w)
	if !ok {
		return
	}
	events := d.Recoveries
	if events == nil {
		events = []core.RecoveryEntry{}
	}
	// These totals reflect operator-entered assertions, NOT bank or provider
	// verification. Current confirmations are evidence-version scoped.
	respond(w, http.StatusOK, map[string]any{
		"events":             events,
		"totals_by_currency": core.RecoveryTotals(d),
		"disclaimer":         "Operator-entered records only. Confirmed does not mean collectible and collected has not been independently verified.",
	})
}

func (s *Server) recordRecovery(w http.ResponseWriter, r *http.Request) {
	var req core.RecoveryInput
	if err := parseJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "Invalid recovery JSON")
		return
	}
	var event core.RecoveryEntry
	err := s.Store.Update(func(d *core.Database) error {
		var insertErr error
		event, insertErr = core.RecordRecovery(d, r.PathValue("id"), req, time.Now())
		return insertErr
	})
	switch {
	case errors.Is(err, core.ErrRecoveryNotFound):
		fail(w, http.StatusNotFound, "Investigation not found")
	case errors.Is(err, core.ErrRecoveryInvalid):
		fail(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, core.ErrRecoveryConflict):
		fail(w, http.StatusConflict, err.Error())
	case err != nil:
		log.Printf("recovery persistence: %v", err)
		fail(w, http.StatusInternalServerError, "Unable to save recovery record")
	default:
		respond(w, http.StatusOK, map[string]any{
			"entry":      event,
			"disclaimer": "Recorded by an operator; no money transfer or independent financial verification has occurred.",
		})
	}
}
