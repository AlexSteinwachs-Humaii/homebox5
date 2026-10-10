package main

import (
	"strings"
	"testing"
)

func TestCSVContractsPreserveWireOptionality(t *testing.T) {
	in := "export interface ServicesCSVCaptureRequest {\n  source: string;\n  collectionId?: string;\n}\nexport interface ServicesCSVArtifact {\n  completedAt: string | null;\n  retryAt?: string | null;\n  createdAt: string;\n}\nexport interface RepoGroup {\n  createdAt?: string;\n}\n"
	out := processContracts(in, []ReReplace{NewReReplace(` Services`, " "), NewReReplace(` Repo`, " "), NewReReplace(`\?:`, ":"), NewReDate("createdAt")})
	for _, want := range []string{"interface CSVCaptureRequest", "collectionId?: string", "completedAt: string | null", "retryAt?: string | null", "createdAt: string;", "interface Group", "createdAt: Date | string"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}
