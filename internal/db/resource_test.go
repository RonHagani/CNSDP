package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsResourceExhausted(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "disk_full 53100 matches",
			err:  &pgconn.PgError{Code: "53100", Message: "could not extend file: No space left on device"},
			want: true,
		},
		{
			name: "disk_full 53100 wrapped several times still matches",
			err:  fmt.Errorf("validation: insert validation outcome for submission 7: %w", fmt.Errorf("submission: admit: %w", &pgconn.PgError{Code: "53100"})),
			want: true,
		},
		{
			name: "out_of_memory 53200 does not match",
			err:  &pgconn.PgError{Code: "53200", Message: "out of memory"},
			want: false,
		},
		{
			name: "too_many_connections 53300 does not match",
			err:  &pgconn.PgError{Code: "53300", Message: "too many connections"},
			want: false,
		},
		{
			name: "unrelated PgError code does not match",
			err:  &pgconn.PgError{Code: "23505", Message: "unique_violation"},
			want: false,
		},
		{
			name: "non-PgError does not match",
			err:  errors.New("some other failure"),
			want: false,
		},
		{
			name: "nil error does not match",
			err:  nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsResourceExhausted(tt.err); got != tt.want {
				t.Errorf("IsResourceExhausted(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
