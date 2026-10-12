package drop

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/PaNasMs/module-file-drop/internal/access"
	"github.com/PaNasMs/module-sdk/auth"
)

func response(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func problem(w http.ResponseWriter, code int, err error) {
	response(w, code, map[string]string{"error": err.Error()})
}

func (s *Service) Handler(allowed map[string]bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := auth.LookupPanel(r.URL.Query().Get("user"), allowed)
		if err != nil {
			problem(w, http.StatusForbidden, access.ErrDenied)
			return
		}
		a, err := account(id)
		if err != nil {
			problem(w, http.StatusForbidden, access.ErrDenied)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/state":
			state, err := s.State(a)
			if err != nil {
				problem(w, http.StatusServiceUnavailable, err)
				return
			}
			response(w, http.StatusOK, state)
		case r.Method == http.MethodPost && r.URL.Path == "/window":
			if err = s.Start(a); err != nil {
				problem(w, http.StatusConflict, err)
				return
			}
			response(w, http.StatusOK, map[string]bool{"receiving": true})
		case r.Method == http.MethodDelete && r.URL.Path == "/window":
			if err = s.Stop(a); err != nil {
				problem(w, http.StatusConflict, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/offer":
			var body struct {
				ID       string `json:"id"`
				Decision string `json:"decision"`
			}
			if !decode(r, &body) || (body.Decision != "accept" && body.Decision != "decline") {
				problem(w, http.StatusBadRequest, errors.New("invalid decision"))
				return
			}
			if err = s.Decide(a, body.ID, body.Decision == "accept"); err != nil {
				problem(w, http.StatusConflict, err)
				return
			}
			response(w, http.StatusOK, map[string]bool{"accepted": body.Decision == "accept"})
		case r.Method == http.MethodPost && r.URL.Path == "/inbox":
			var body struct {
				Path string `json:"path"`
			}
			if !decode(r, &body) {
				problem(w, http.StatusBadRequest, errors.New("invalid destination"))
				return
			}
			if err = s.SetInbox(a, body.Path); err != nil {
				problem(w, http.StatusBadRequest, err)
				return
			}
			response(w, http.StatusOK, map[string]string{"path": body.Path})
		default:
			problem(w, http.StatusNotFound, errors.New("not found"))
		}
	})
}

func decode(r *http.Request, value any) bool {
	defer r.Body.Close()
	d := json.NewDecoder(io.LimitReader(r.Body, 8193))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}
