package dsprocessors

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"gostreambridge/pkg/util"
)

func WriteToMySQL(message string) error {

	// Reading configuration
	mysqlConfig := util.ConvertConfigFileToMap("mysql.json")

	// Construct the DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", mysqlConfig["username"], mysqlConfig["password"], mysqlConfig["host"], mysqlConfig["port"], mysqlConfig["database"])

	// Connect to MySQL database
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error connecting to MySQL database: %v", err)
	}
	defer db.Close()

	// Prepare statement to insert message into table with timestamp
	stmt, err := db.Prepare("INSERT INTO messages (message, timestamp) VALUES (?, ?)")
	if err != nil {
		log.Fatalf("Error preparing MySQL statement: %v", err)
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
