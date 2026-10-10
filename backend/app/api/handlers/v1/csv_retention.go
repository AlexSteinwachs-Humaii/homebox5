package v1

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/hay-kot/httpkit/server"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
)

func (ctrl *V1Controller) HandleCSVRetention(private, update bool) errchain.HandlerFunc {
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
			if errors.Is(err, services.ErrCSVAccess) {
				status, code = http.StatusNotFound, "csv_unavailable"
			}
			return server.JSON(w, status, map[string]string{"code": code})
		}
		if update {
			// Decode into an integer and reject unknown, duplicate and trailing fields.
			raw, err := io.ReadAll(io.LimitReader(r.Body, 1025))
			var fields map[string]json.RawMessage
			if err != nil || len(raw) > 1024 || json.Unmarshal(raw, &fields) != nil || len(fields) != 1 || fields["days"] == nil {
				return server.JSON(w, http.StatusBadRequest, map[string]string{"code": "csv_invalid_retention"})
			}
			var policy services.CSVRetentionPolicy
			// Comparing object tokens catches duplicate keys that Unmarshal would ignore.
			dec := json.NewDecoder(strings.NewReader(string(raw)))
			_, err = dec.Token()
			count := 0
			for err == nil && dec.More() {
				_, err = dec.Token()
				if err != nil {
					break
				}
				var value json.RawMessage
				err = dec.Decode(&value)
				count++
			}
			if count != 1 || err != nil || json.Unmarshal(raw, &policy) != nil || repo.ValidateCSVRetentionDays(policy.Days) != nil {
				return server.JSON(w, http.StatusBadRequest, map[string]string{"code": "csv_invalid_retention"})
			}
			if err := ctrl.svc.CSVExports.SetRetention(ctx, ctx.UID, gid, policy.Days); err != nil {
				return fail(err)
			}
		}
		policy, err := ctrl.svc.CSVExports.Retention(ctx, ctx.UID, gid)
		if err != nil {
			return fail(err)
		}
		return server.JSON(w, http.StatusOK, policy)
	}
}
