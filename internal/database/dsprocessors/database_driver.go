package dsprocessors

import (
	"database/sql"
	"fmt"
	"gostreambridge/pkg/util"
	"log"
	"time"
)

func WriteToDatabase(driver, configFile, message string) error {
	// Read database configuration from file
	dbConfig, err := util.ConvertConfigFileToMap(configFile)
	if err != nil {
		return fmt.Errorf("error reading %s config file: %v", driver, err)
	}

	// Construct DSN/connection string
	var dsn string
	switch driver {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbConfig["username"], dbConfig["password"], dbConfig["host"], dbConfig["port"], dbConfig["database"])
	case "sqlserver":
		dsn = fmt.Sprintf("server=%s;user id=%s;password=%s;port=%s;database=%s", dbConfig["host"], dbConfig["username"], dbConfig["password"], dbConfig["port"], dbConfig["database"])
	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}

	// Connect to the database
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("error connecting to %s database: %v", driver, err)
	}
	defer db.Close()

	// Prepare statement to insert message into table with timestamp
	stmt, err := db.Prepare("INSERT INTO messages (message, timestamp) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("error preparing %s statement: %v", driver, err)
	}
	defer stmt.Close()

	// Execute the prepared statement with the message and current timestamp as parameters with retry
	maxRetries := 5 // Number of retry attempts
	for i := 0; i < maxRetries; i++ {
		_, err = stmt.Exec(message, time.Now())
		if err == nil {
			log.Printf("Message written to %s: %s\n", driver, message)
			return nil
		}
		log.Printf("Error executing %s statement: %v, retrying...\n", driver, err)
		time.Sleep(2 * time.Second) // Wait before retrying
	}

	return fmt.Errorf("error executing %s statement after %d retries: %v", driver, maxRetries, err)
}
