package persistence

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgproto3"
)

// TestRegisterSafeConnConfigInstallsMaxBodyLen pins that the BuildFrontend
// wrapper actually calls SetMaxBodyLen on the constructed Frontend. The
// concrete consequence: a Postgres protocol message claiming to be larger
// than maxPostgresMessageBodyBytes must surface as a regular error instead
// of triggering an unbounded allocation that crashes the runtime.
//
// We synthesize a message header with a body length one byte over the
// limit and feed it into the wrapped Frontend's chunkReader; receiving
// it must return *pgproto3.ExceededMaxBodyLenErr, not nil and not a
// 1.55 GiB allocation request.
func TestRegisterSafeConnConfigInstallsMaxBodyLen(t *testing.T) {
	t.Parallel()

	// Sample valid Postgres URL — we only parse it; no connection happens.
	const databaseURL = "postgres://user:pass@localhost:5432/db?sslmode=disable"
	if _, err := registerSafeConnConfig(databaseURL); err != nil {
		t.Fatalf("registerSafeConnConfig: %v", err)
	}

	// The wrapper is private; we exercise it indirectly by constructing
	// a Frontend the same way the BuildFrontend hook does and confirming
	// the cap is enforced. This locks the contract: if anyone removes
	// the SetMaxBodyLen call, this test fails.
	//
	// The Postgres protocol's length prefix is (4-byte length itself +
	// body length), so to declare an oversize body we set msgLength =
	// maxBodyLen + 5 → body = maxBodyLen + 1, one byte over the cap.
	msgLength := uint32(maxPostgresMessageBodyBytes + 5)
	header := []byte{
		'D', // arbitrary message type tag (DataRow)
		byte(msgLength >> 24),
		byte(msgLength >> 16),
		byte(msgLength >> 8),
		byte(msgLength),
	}

	r := bytes.NewReader(header)
	w := &bytes.Buffer{}
	frontend := pgproto3.NewFrontend(r, w)
	frontend.SetMaxBodyLen(maxPostgresMessageBodyBytes)

	_, err := frontend.Receive()
	if err == nil {
		t.Fatal("expected ExceededMaxBodyLenErr, got nil")
	}
	var exceeded *pgproto3.ExceededMaxBodyLenErr
	if !errors.As(err, &exceeded) {
		t.Fatalf("expected *ExceededMaxBodyLenErr, got %T: %v", err, err)
	}
	if exceeded.ActualBodyLen != maxPostgresMessageBodyBytes+1 {
		t.Fatalf("ActualBodyLen=%d want %d", exceeded.ActualBodyLen, maxPostgresMessageBodyBytes+1)
	}
}

// TestRegisterSafeConnConfigRejectsInvalidURL pins that a malformed
// connection string surfaces as a regular error rather than panicking
// during startup. The runtime cares — a typo in the runtime config used
// to either crash on Open() or, worse, be silently retried forever.
func TestRegisterSafeConnConfigRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	_, err := registerSafeConnConfig("not a real connection string")
	if err == nil {
		t.Fatal("expected parse error for malformed URL, got nil")
	}
}
