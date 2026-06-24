package core

import (
	"errors"
	"testing"
)

func TestErrors_Operational(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		op   string
		err  error
	}{
		{"Network Timeout", "fetch", errors.New("timeout")},
		{"DB Error", "query", errors.New("connection reset")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opErr := &OperationalError{Op: tt.op, Err: tt.err}

			if !errors.Is(opErr, tt.err) {
				t.Fatalf("expected opErr to unwrap to inner error")
			}

			var target *OperationalError
			if !errors.As(opErr, &target) {
				t.Fatalf("expected errors.As to match OperationalError")
			}
			if target.Op != tt.op {
				t.Fatalf("expected op %s, got %s", tt.op, target.Op)
			}
		})
	}
}

func TestErrors_Validation(t *testing.T) {
	t.Parallel()
	valErr := &ValidationError{Field: "amount", Reason: "must be positive"}

	var target *ValidationError
	if !errors.As(valErr, &target) {
		t.Fatal("expected errors.As to match ValidationError")
	}
}
