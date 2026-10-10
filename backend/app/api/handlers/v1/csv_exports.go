package v1

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/hay-kot/httpkit/server"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
)

// HandleCSV is shared by collection and account-private routes. Private routes
// deliberately omit tenant middleware; the authenticated UID is the only scope.
// The route selects the operation: no caller-supplied status/blob key is accepted.
func (ctrl *V1Controller) HandleCSV(private bool, operation string) errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		ctx := services.NewContext(r.Context())
		var gid *uuid.UUID
		if !private {
			gid = &ctx.GID
		}
		fail := func(err error) error {
			status, code := http.StatusInternalServerError, "csv_internal"
			switch {
			case errors.Is(err, services.ErrCSVAccess):
				status, code = http.StatusNotFound, "csv_unavailable"
			case errors.Is(err, services.ErrCSVExpired):
				status, code = http.StatusGone, "csv_expired"
			case errors.Is(err, services.ErrCSVNotReady):
				status, code = http.StatusConflict, "csv_not_ready"
			case errors.Is(err, services.ErrCSVStorage):
				status, code = http.StatusServiceUnavailable, "csv_storage_unavailable"
			}
			return server.JSON(w, status, CSVError{Code: code})
		}
		if operation == "create" {
			capture, err := services.DecodeCSVCapture(r.Body)
			if err != nil {
				return server.JSON(w, http.StatusBadRequest, CSVError{Code: "csv_invalid_capture"})
			}
			if (private && capture.CollectionID != nil) || (!private && (capture.CollectionID == nil || *capture.CollectionID != ctx.GID)) {
				return fail(services.ErrCSVAccess)
			}
			row, err := ctrl.svc.CSVExports.Enqueue(ctx, ctx.UID, capture)
			// Capture failures never expose raw source, SQL or storage diagnostics.
			if err != nil {
				return fail(services.ErrCSVAccess)
			}
			return server.JSON(w, http.StatusAccepted, services.CSVArtifactView(row))
		}
		if operation == "history" {
			out, err := ctrl.svc.CSVExports.History(ctx, ctx.UID, gid)
			if err != nil {
				return fail(err)
			}
			return server.JSON(w, http.StatusOK, WrapResults(out))
		}
		id, err := ctrl.routeID(r)
		if err != nil {
			return server.JSON(w, http.StatusBadRequest, CSVError{Code: "csv_invalid_id"})
		}
		switch operation {
		case "status":
			out, err := ctrl.svc.CSVExports.Status(ctx, ctx.UID, gid, id)
			if err != nil {
				return fail(err)
			}
			return server.JSON(w, http.StatusOK, out)
		case "retry":
			if err := ctrl.svc.CSVExports.RetryArtifact(ctx, ctx.UID, gid, id); err != nil {
				return fail(err)
			}
			w.WriteHeader(http.StatusAccepted)
			return nil
		case "delete":
			if err := ctrl.svc.CSVExports.Delete(ctx, ctx.UID, gid, id); err != nil {
				return fail(err)
			}
			// Accepted means durable deletion intent, not unconfirmed blob removal.
			w.WriteHeader(http.StatusAccepted)
			return nil
		case "download":
			data, err := ctrl.svc.CSVExports.Download(ctx, ctx.UID, gid, id)
			if err != nil {
				return fail(err)
			}
			allowSlowResponse(w, r)
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="homebox-csv-%s.csv"`, id))
			w.Header().Set("Content-Length", fmt.Sprint(len(data)))
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(data)
			// Network/local-save failures are not worker lifecycle mutations.
			return err
		}
		return fail(services.ErrCSVAccess)
	}
}
