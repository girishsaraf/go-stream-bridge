package sink

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	mssql "github.com/microsoft/go-mssqldb"
)

func TestClassifySQLError(t *testing.T) {
	tests := []struct {
		name      string
		driver    string
		err       error
		permanent bool
		verify    func(error) bool
	}{
		{
			name:      "mysql syntax error",
			driver:    "mysql",
			err:       &mysql.MySQLError{Number: 1064, Message: "syntax"},
			permanent: true,
		},
		{
			name:      "mysql duplicate entry",
			driver:    "mysql",
			err:       &mysql.MySQLError{Number: 1062, Message: "duplicate"},
			permanent: true,
		},
		{
			name:   "mysql deadlock is retryable",
			driver: "mysql",
			err:    &mysql.MySQLError{Number: 1213, Message: "deadlock"},
		},
		{
			name:      "sqlserver invalid object",
			driver:    "sqlserver",
			err:       mssql.Error{Number: 208, Message: "invalid object"},
			permanent: true,
			verify: func(err error) bool {
				var sqlErr mssql.Error
				return errors.As(err, &sqlErr) && sqlErr.Number == 208
			},
		},
		{
			name:   "sqlserver deadlock is retryable",
			driver: "sqlserver",
			err:    mssql.Error{Number: 1205, Message: "deadlock"},
			verify: func(err error) bool {
				var sqlErr mssql.Error
				return errors.As(err, &sqlErr) && sqlErr.Number == 1205
			},
		},
		{
			name:   "network error is retryable",
			driver: "mysql",
			err:    errors.New("connection reset"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := classifySQLError(test.driver, test.err)
			verify := test.verify
			if verify == nil {
				verify = func(err error) bool { return errors.Is(err, test.err) }
			}
			if !verify(err) {
				t.Fatalf("classification lost the original error: %v", err)
			}
			if got := IsPermanent(err); got != test.permanent {
				t.Fatalf("permanent=%v, want %v", got, test.permanent)
			}
		})
	}
}

func TestClassifySQLErrorNil(t *testing.T) {
	if err := classifySQLError("mysql", nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
