package main

import (
	"errors"
	"strings"
	"testing"
)

// TestBootstrapStmtErr_NeverLeaksPassword guards against the CodeQL
// "clear-text logging of sensitive information" finding this test file
// accompanies: a bootstrap statement's own SQL text (which, for statement
// 0, embeds the generated read-only role's password) must never reach an
// error returned by this package, since every phase error is logged
// verbatim and persisted into the JSON report (main.go's recordPhase).
func TestBootstrapStmtErr_NeverLeaksPassword(t *testing.T) {
	const fakePassword = "deadbeefcafef00dfeedfacef00dcafe"

	stmts := bootstrapStatements(fakePassword)
	if len(stmts) != len(bootstrapStmtNames) {
		t.Fatalf("bootstrapStatements returned %d statements, bootstrapStmtNames has %d", len(stmts), len(bootstrapStmtNames))
	}

	// Sanity precondition: statement 0 genuinely does contain the secret,
	// so this test is exercising a real leak path, not a vacuous one.
	if !strings.Contains(stmts[0], fakePassword) {
		t.Fatalf("test precondition failed: bootstrapStatements()[0] does not contain the password -- test would not detect a regression")
	}

	driverErr := errors.New("some driver failure")
	for i := range stmts {
		err := bootstrapStmtErr(i, driverErr)
		if err == nil {
			t.Fatalf("bootstrapStmtErr(%d, ...) returned nil", i)
		}
		msg := err.Error()
		if strings.Contains(msg, fakePassword) {
			t.Errorf("bootstrapStmtErr(%d, ...) leaked the password into its error message: %q", i, msg)
		}
		if strings.Contains(msg, stmts[i]) {
			t.Errorf("bootstrapStmtErr(%d, ...) leaked the raw statement body into its error message: %q", i, msg)
		}
		wantSubstr := bootstrapStmtNames[i]
		if !strings.Contains(msg, wantSubstr) {
			t.Errorf("bootstrapStmtErr(%d, ...) = %q, want it to identify the failing step as %q", i, msg, wantSubstr)
		}
	}
}
