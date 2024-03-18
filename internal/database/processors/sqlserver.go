package sqlserver

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/denisenkom/go-mssqldb"

	"gostreambridge/internal/config"
)


// WriteToSQLServer writes messages to SQL Server database
func WriteToSQLServer(message string) error {

	var sqlServerConfig config.SQLServerConfig
	// Reading configuration
	sqlServerConfig = util.ConvertConfigFileToMap("sqlserver.json")

	// Construct the connection string
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%s;database=%s", sqlServerConfig.Host, sqlServerConfig.Username, sqlServerConfig.Password, sqlServerConfig.Port, sqlServerConfig.Database)

	// Connect to SQL Server database
	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		return fmt.Errorf("error connecting to SQL Server database: %v", err)
	}
	defer db.Close()

	// Prepare statement to insert message into table with timestamp
	stmt, err := db.Prepare("INSERT INTO messages (message, timestamp) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("error preparing SQL Server statement: %v", err)
	}
	defer stmt.Close()

	// Execute the prepared statement with the message and current timestamp as parameters
	_, err = stmt.Exec(message, time.Now())
	if err != nil {
		return fmt.Errorf("error executing SQL Server statement: %v", err)
	}

	log.Printf("Message written to SQL Server: %s\n", message)
	return nil
}
