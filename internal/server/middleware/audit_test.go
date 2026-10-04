package middleware

import (
	"net/http"
	"testing"
)

// TestShouldAuditManagementWrite_PoolAccountExportRoute locks in the audit
// coverage of the pool account export route (B2-#2 step 7, audit-2 P0-1/P2-3).
//
// Regression context: export was originally registered as GET. Adding a GET
// entry to auditedManagementWriteRoutes has no effect at all, because
// isPotentialAuditRequest short-circuits every non-POST/PUT/PATCH/DELETE
// method before the whitelist is consulted — the route was silently unaudited.
// Export is therefore a POST now; this test fails if the whitelist entry is
// dropped, or if the route is reverted to a non-auditable method.
func TestShouldAuditManagementWrite_PoolAccountExportRoute(t *testing.T) {
	const exportFullPath = "/api/v1/pool/:id/account/export"

	if !ShouldAuditManagementWrite(http.MethodPost, exportFullPath) {
		t.Fatalf("POST %s must be covered by the audit whitelist", exportFullPath)
	}

	// The GET form must stay unauditable: isPotentialAuditRequest rejects
	// every non-writing method before the whitelist lookup. If this ever
	// returns true, the short-circuit was removed and read endpoints would
	// start flooding the audit log.
	if isPotentialAuditRequest(http.MethodGet, "/api/v1/pool/1/account/export") {
		t.Fatalf("GET requests must be short-circuited by isPotentialAuditRequest before the whitelist lookup")
	}
	if ShouldAuditManagementWrite(http.MethodGet, exportFullPath) {
		t.Fatalf("GET must never match the audit whitelist")
	}
}
