package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	
	"gostreambridge/internal/config"
	"gostreambridge/pkg/util"
)

func WriteToMySQL(message string) error {

	// Reading configuration
	mysqlConfig = util.ConvertConfigFileToMap("mysql.json")
	
	// Construct the DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", mysqlConfig["username"], mysqlConfig["password"], mysqlConfig["host"], mysqlConfig["port"], mysqlConfig["database"])

	// Connect to MySQL database
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return log.Errorf("Error connecting to MySQL database: %v", err)
	}
	defer db.Close()

	// Prepare statement to insert message into table with timestamp
	stmt, err := db.Prepare("INSERT INTO messages (message, timestamp) VALUES (?, ?)")
	if err != nil {
		return log.Errorf("Error preparing MySQL statement: %v", err)
	}
	defer stmt.Close()

	// Execute the prepared statement with the message and current timestamp as parameters
	_, err = stmt.Exec(message, time.Now())
	if err != nil {
		return log.Errorf("Error executing MySQL statement: %v", err)
	}

	log.Printf("Message written to MySQL: %s\n", message)
	return nil
}
