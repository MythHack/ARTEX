package server

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestIsProfileRowGone locks the FK-violation classification used to silence
// breaker persistence for deleted profiles: only 23503 counts, and it must be
// detected through wrapping.
func TestIsProfileRowGone(t *testing.T) {
	fk := &pgconn.PgError{Code: "23503", ConstraintName: "llm_profile_health_profile_id_fkey"}
	if !isProfileRowGone(errors.Join(errors.New("write failed"), fk)) {
		t.Fatal("wrapped 23503 not classified as profile-gone")
	}
	if isProfileRowGone(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("unique violation misclassified as profile-gone")
	}
	if isProfileRowGone(errors.New("connection refused")) {
		t.Fatal("network error misclassified as profile-gone")
	}
}
