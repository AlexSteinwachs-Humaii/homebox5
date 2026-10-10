import { BaseAPI, route } from "../base";
import type { CsvArtifact, CsvRequest } from "../types/csv-contracts.generated";
import type { CsvExportRequest } from "../../exports/source-registry";

export class CsvExportsAPI extends BaseAPI {
  private path(scope: CsvRequest["scope"]) {
    return scope === "private" ? "/user/csv-exports" : "/group/csv-exports";
  }

  create(request: CsvExportRequest) {
    return this.http.post<CsvRequest, CsvArtifact>({
      url: route(this.path(request.scope)),
      body: request,
    });
  }

  get(scope: CsvRequest["scope"], id: string) {
    return this.http.get<CsvArtifact>({
      url: route(`${this.path(scope)}/${encodeURIComponent(id)}`),
    });
  }

  download(scope: CsvRequest["scope"], id: string) {
    return this.http.get<ReadableStream<Uint8Array>>({
      url: route(`${this.path(scope)}/${encodeURIComponent(id)}/download`),
    });
  }
}
