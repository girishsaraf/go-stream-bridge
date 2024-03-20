package dsprocessors

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"gostreambridge/pkg/util"
)

// WriteToSQLServer writes messages to SQL Server database
func WriteToSQLServer(message string) error {

	// Reading configuration
	sqlServerConfig := util.ConvertConfigFileToMap("sqlserver.json")

	// Construct the connection string
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%s;database=%s", sqlServerConfig["host"], sqlServerConfig["username"], sqlServerConfig["password"], sqlServerConfig["port"], sqlServerConfig["database"])

	// Connect to SQL Server database
	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		log.Fatalf("Error connecting to SQL Server database: %v", err)
	}
	defer db.Close()

	// Prepare statement to insert message into table with timestamp
	stmt, err := db.Prepare("INSERT INTO messages (message, timestamp) VALUES (?, ?)")
	if err != nil {
		log.Fatalf("Error preparing SQL Server statement: %v", err)
	}
	defer stmt.Close()

	// Execute the prepared statement with the message and current timestamp as parameters with retry
	maxRetries := 5 // Number of retry attempts
	for i := 0; i < maxRetries; i++ {
		_, err = stmt.Exec(message, time.Now())
		if err == nil {
			log.Printf("Message written to MySQL: %s\n", message)
			return nil
		}
		log.Printf("Error executing MySQL statement: %v, retrying...\n", err)
		time.Sleep(2 * time.Second) // Wait before retrying
	}

	return fmt.Errorf("error executing MySQL statement after %d retries: %v", maxRetries, err)
}
