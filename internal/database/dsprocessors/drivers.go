// Package dsprocessors contains downstream data-store writers.
package dsprocessors

import (
	// Register the SQL drivers used by WriteToDatabase.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/microsoft/go-mssqldb"
)
