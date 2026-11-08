package sink

import (
	// Register the SQL drivers used by the database sinks.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/microsoft/go-mssqldb"
)
