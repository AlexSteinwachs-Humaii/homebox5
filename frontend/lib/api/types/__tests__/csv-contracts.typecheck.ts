// Compile-only integration fixture: no UI, encoder or duplicated policy types.
import type { CSVCaptureRequest, CSVArtifact, CSVRetentionPolicy, ResultsServicesCSVArtifact } from "../data-contracts";

export const privateCapture: CSVCaptureRequest = {
  source: "account-api-keys",
  scope: "account-private",
  columns: ["id", "name"],
};
export const collectionCapture: CSVCaptureRequest = {
  source: "items",
  scope: "collection",
  collectionId: "d1c5477b-827b-4889-8aaf-9f22ce465c9e",
  columns: ["name"],
  entities: { search: "desk" },
  ordering: [{ column: "name", direction: "asc" }],
};
export const policy: CSVRetentionPolicy = { days: 30 };
export function lifecycle(history: ResultsServicesCSVArtifact): CSVArtifact | undefined {
  return history.items?.[0];
}
// @ts-expect-error Static choice lists are not typed CSV sources.
export const unsupported: CSVCaptureRequest["source"] = "themes";
// @ts-expect-error Arbitrary SQL ordering is not a supported direction.
export const unsupportedDirection: NonNullable<CSVCaptureRequest["ordering"]>[number]["direction"] = "SQL";
// @ts-expect-error Secrets are never a typed export column.
export const secretColumn: CSVCaptureRequest["columns"][number] = "token";
