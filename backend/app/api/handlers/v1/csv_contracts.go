package v1

import "github.com/hay-kot/httpkit/errchain"

// CSVError is a safe public error; it never contains SQL, captures, secrets or blob keys.
type CSVError struct {
	Code string `validate:"required" json:"code" enums:"csv_invalid_capture,csv_invalid_id,csv_invalid_retention,csv_unavailable,csv_expired,csv_not_ready,csv_storage_unavailable,csv_internal"`
}

// HandleCSVCreate documents both scopes and delegates to the same service boundary.
// @Summary Capture and enqueue a CSV export
// @Tags CSV exports
// @Description Collection routes require current membership and source permissions; X-Tenant selects the collection. Private routes need no tenant and use the authenticated user only. Errors use safe codes; unavailable authorization is indistinguishable from missing data (404). Unknown/duplicate keys, unsupported columns and pagination are rejected. Source-dependent rules are defined in the CSV consumption guide and source coverage matrix.
// @Produce json
// @Param X-Tenant header string false "Collection UUID for collection routes only; capture.collectionId must match"
// @Accept json
// @Param capture body services.CSVCaptureRequest true "Validated source capture"
// @Success 202 {object} services.CSVArtifact "Success"
// @Failure 400 {object} CSVError
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} CSVError
// @Failure 500 {object} CSVError
// @Router /v1/group/csv-exports [post]
// @Router /v1/users/self/csv-exports [post]
// @Security Bearer
func (ctrl *V1Controller) HandleCSVCreate(private bool) errchain.HandlerFunc {
	return ctrl.HandleCSV(private, "create")
}

// HandleCSVHistory documents both scopes and delegates to the same service boundary.
// @Summary List currently authorized CSV artifacts
// @Tags CSV exports
// @Description Collection routes require current membership and source permissions; X-Tenant selects the collection. Private routes need no tenant and use the authenticated user only. Errors use safe codes; unavailable authorization is indistinguishable from missing data (404).
// @Produce json
// @Param X-Tenant header string false "Collection UUID for collection routes only; capture.collectionId must match"
// @Success 200 {object} Results[services.CSVArtifact] "Success"
// @Failure 400 {object} CSVError
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} CSVError
// @Failure 500 {object} CSVError
// @Router /v1/group/csv-exports [get]
// @Router /v1/users/self/csv-exports [get]
// @Security Bearer
func (ctrl *V1Controller) HandleCSVHistory(private bool) errchain.HandlerFunc {
	return ctrl.HandleCSV(private, "history")
}

// HandleCSVStatus documents both scopes and delegates to the same service boundary.
// @Summary Observe CSV lifecycle
// @Tags CSV exports
// @Description Collection routes require current membership and source permissions; X-Tenant selects the collection. Private routes need no tenant and use the authenticated user only. Errors use safe codes; unavailable authorization is indistinguishable from missing data (404).
// @Produce json
// @Param X-Tenant header string false "Collection UUID for collection routes only; capture.collectionId must match"
// @Param id path string true "Artifact UUID" format(uuid)
// @Success 200 {object} services.CSVArtifact "Success"
// @Failure 400 {object} CSVError
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} CSVError
// @Failure 500 {object} CSVError
// @Router /v1/group/csv-exports/{id} [get]
// @Router /v1/users/self/csv-exports/{id} [get]
// @Security Bearer
func (ctrl *V1Controller) HandleCSVStatus(private bool) errchain.HandlerFunc {
	return ctrl.HandleCSV(private, "status")
}

// HandleCSVDownload documents both scopes and delegates to the same service boundary.
// @Summary Download canonical retained CSV bytes
// @Tags CSV exports
// @Description Collection routes require current membership and source permissions; X-Tenant selects the collection. Private routes need no tenant and use the authenticated user only. Errors use safe codes; unavailable authorization is indistinguishable from missing data (404). Returns text/csv UTF-8 attachment bytes, not JSON or a redirect. Authorization is rechecked and freshly generated canonical bytes must match retained bytes; changed data can deny download. Expiry is enforced at or after expiresAt (410). Local-save/network failures do not change server lifecycle.
// @Produce text/csv
// @Param X-Tenant header string false "Collection UUID for collection routes only; capture.collectionId must match"
// @Param id path string true "Artifact UUID" format(uuid)
// @Success 200 {file} file "Success"
// @Failure 400 {object} CSVError
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} CSVError
// @Failure 500 {object} CSVError
// @Failure 409 {object} CSVError
// @Failure 410 {object} CSVError
// @Failure 503 {object} CSVError
// @Router /v1/group/csv-exports/{id}/download [get]
// @Router /v1/users/self/csv-exports/{id}/download [get]
// @Security Bearer
func (ctrl *V1Controller) HandleCSVDownload(private bool) errchain.HandlerFunc {
	return ctrl.HandleCSV(private, "download")
}

// HandleCSVRetry documents both scopes and delegates to the same service boundary.
// @Summary Accept an explicit requester retry
// @Tags CSV exports
// @Description Collection routes require current membership and source permissions; X-Tenant selects the collection. Private routes need no tenant and use the authenticated user only. Errors use safe codes; unavailable authorization is indistinguishable from missing data (404). 202 has an empty body. Only the requester may retry generation-failed or storage-failed; other states return 409.
// @Produce json
// @Param X-Tenant header string false "Collection UUID for collection routes only; capture.collectionId must match"
// @Param id path string true "Artifact UUID" format(uuid)
// @Success 202 "Empty body"
// @Failure 400 {object} CSVError
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} CSVError
// @Failure 500 {object} CSVError
// @Failure 409 {object} CSVError
// @Router /v1/group/csv-exports/{id}/retry [post]
// @Router /v1/users/self/csv-exports/{id}/retry [post]
// @Security Bearer
func (ctrl *V1Controller) HandleCSVRetry(private bool) errchain.HandlerFunc {
	return ctrl.HandleCSV(private, "retry")
}

// HandleCSVDelete documents both scopes and delegates to the same service boundary.
// @Summary Accept durable deletion intent
// @Tags CSV exports
// @Description Collection routes require current membership and source permissions; X-Tenant selects the collection. Private routes need no tenant and use the authenticated user only. Errors use safe codes; unavailable authorization is indistinguishable from missing data (404). 202 has an empty body and means cleanup-pending, not confirmed blob removal; repeated scoped missing deletion succeeds.
// @Produce json
// @Param X-Tenant header string false "Collection UUID for collection routes only; capture.collectionId must match"
// @Param id path string true "Artifact UUID" format(uuid)
// @Success 202 "Empty body"
// @Failure 400 {object} CSVError
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} CSVError
// @Failure 500 {object} CSVError
// @Router /v1/group/csv-exports/{id} [delete]
// @Router /v1/users/self/csv-exports/{id} [delete]
// @Security Bearer
func (ctrl *V1Controller) HandleCSVDelete(private bool) errchain.HandlerFunc {
	return ctrl.HandleCSV(private, "delete")
}

// HandleCSVRetentionGet documents both scopes and delegates to the same service boundary.
// @Summary Read CSV retention policy
// @Tags CSV exports
// @Description Collection routes require current membership and source permissions; X-Tenant selects the collection. Private routes need no tenant and use the authenticated user only. Errors use safe codes; unavailable authorization is indistinguishable from missing data (404). Collection policy requires current ownership; private policy works without a collection. Default 30, positive whole days up to 106751. Policy is selected at completion and expiry is fixed thereafter.
// @Produce json
// @Param X-Tenant header string false "Collection UUID for collection routes only; capture.collectionId must match"
// @Success 200 {object} services.CSVRetentionPolicy "Success"
// @Failure 400 {object} CSVError
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} CSVError
// @Failure 500 {object} CSVError
// @Router /v1/group/csv-exports/retention [get]
// @Router /v1/users/self/csv-exports/retention [get]
// @Security Bearer
func (ctrl *V1Controller) HandleCSVRetentionGet(private bool) errchain.HandlerFunc {
	return ctrl.HandleCSVRetention(private, false)
}

// HandleCSVRetentionPut documents both scopes and delegates to the same service boundary.
// @Summary Configure CSV retention policy
// @Tags CSV exports
// @Description Collection routes require current membership and source permissions; X-Tenant selects the collection. Private routes need no tenant and use the authenticated user only. Errors use safe codes; unavailable authorization is indistinguishable from missing data (404). Collection policy requires current ownership; private policy works without a collection. Default 30, positive whole days up to 106751. Policy is selected at completion and expiry is fixed thereafter.
// @Produce json
// @Param X-Tenant header string false "Collection UUID for collection routes only; capture.collectionId must match"
// @Accept json
// @Param policy body services.CSVRetentionPolicy true "Whole-day retention"
// @Success 200 {object} services.CSVRetentionPolicy "Success"
// @Failure 400 {object} CSVError
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} CSVError
// @Failure 500 {object} CSVError
// @Router /v1/group/csv-exports/retention [put]
// @Router /v1/users/self/csv-exports/retention [put]
// @Security Bearer
func (ctrl *V1Controller) HandleCSVRetentionPut(private bool) errchain.HandlerFunc {
	return ctrl.HandleCSVRetention(private, true)
}
