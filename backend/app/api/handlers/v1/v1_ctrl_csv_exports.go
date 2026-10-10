package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/hay-kot/httpkit/server"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
)

func csvHTTPError(e error) error {
	if e == nil {
		return nil
	}
	code := http.StatusInternalServerError
	switch {
	case ent.IsNotFound(e):
		code = http.StatusNotFound
	case errors.Is(e, services.ErrCSVAccess):
		code = http.StatusForbidden
	case errors.Is(e, services.ErrCSVExpired):
		code = http.StatusGone
	case errors.Is(e, services.ErrCSVNotReady):
		code = http.StatusConflict
	}
	// Storage/DB diagnostics never escape through the API.
	if code == http.StatusInternalServerError {
		e = errors.New("CSV operation failed")
	}
	return validate.NewRequestError(e, code)
}

// HandleCSVExports uses separate account and collection routes. Private routes
// deliberately ignore X-Tenant and never inherit NewContext's default group.
func (ctrl *V1Controller) HandleCSVExports(private bool, operation string) errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		auth := services.NewContext(r.Context())
		uid := auth.UID
		var gid *uuid.UUID
		if !private {
			gid = &auth.GID
		}
		svc := ctrl.svc.CSVExports
		if operation == "retention" {
			if !private && r.Method == http.MethodPut {
				if e := ctrl.repo.CsvExports.IsOwner(r.Context(), uid, *gid); e != nil {
					return csvHTTPError(services.ErrCSVAccess)
				}
			}
			if r.Method == http.MethodPut {
				var settings csvexports.RetentionSettings
				dec := json.NewDecoder(io.LimitReader(r.Body, 1024))
				dec.DisallowUnknownFields()
				if e := dec.Decode(&settings); e != nil {
					return validate.NewRequestError(e, http.StatusBadRequest)
				}
				var extra any
				if e := dec.Decode(&extra); e != io.EOF {
					return validate.NewRequestError(errors.New("invalid retention document"), http.StatusBadRequest)
				}
				if e := settings.Validate(); e != nil {
					return validate.NewRequestError(e, http.StatusBadRequest)
				}
				if e := ctrl.repo.CsvExports.SetRetention(r.Context(), uid, gid, settings); e != nil {
					return csvHTTPError(e)
				}
			}
			days, e := ctrl.repo.CsvExports.Retention(r.Context(), uid, gid)
			if e != nil {
				return csvHTTPError(e)
			}
			return server.JSON(w, http.StatusOK, csvexports.RetentionSettings{Days: days})
		}
		if operation == "list" {
			if r.Method == http.MethodPost {
				req, e := csvexports.DecodeRequest(r.Body)
				if e != nil {
					return validate.NewRequestError(e, http.StatusBadRequest)
				}
				out, e := svc.Create(r.Context(), uid, gid, req)
				if e != nil {
					return csvHTTPError(e)
				}
				return server.JSON(w, http.StatusAccepted, out)
			}
			out, e := svc.List(r.Context(), uid, gid)
			if e != nil {
				return csvHTTPError(e)
			}
			return server.JSON(w, http.StatusOK, WrapResults(out))
		}
		id, e := ctrl.routeID(r)
		if e != nil {
			return e
		}
		switch operation {
		case "download":
			reader, out, e := svc.Download(r.Context(), uid, gid, id)
			if e != nil {
				return csvHTTPError(e)
			}
			defer reader.Close()
			allowSlowResponse(w, r)
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="homebox-csv-%s.csv"`, id))
			w.Header().Set("Content-Length", fmt.Sprint(out.SizeBytes))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, e = io.Copy(w, reader)
			return e
		case "retry":
			if e := svc.Retry(r.Context(), uid, gid, id); e != nil {
				return csvHTTPError(e)
			}
			w.WriteHeader(http.StatusAccepted)
			return nil
		default:
			if r.Method == http.MethodDelete {
				if e := svc.Delete(r.Context(), uid, gid, id); e != nil {
					return csvHTTPError(e)
				}
				w.WriteHeader(http.StatusNoContent)
				return nil
			}
			out, e := svc.Get(r.Context(), uid, gid, id)
			if e != nil {
				return csvHTTPError(e)
			}
			return server.JSON(w, http.StatusOK, out)
		}
	}
}
