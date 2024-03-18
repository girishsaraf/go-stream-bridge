package database

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

	// Execute the prepared statement with the message and current timestamp as parameters
	_, err = stmt.Exec(message, time.Now())
	if err != nil {
		log.Fatalf("Error executing SQL Server statement: %v", err)
	}

	log.Printf("Message written to SQL Server: %s\n", message)
	return nil
}
