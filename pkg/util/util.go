package util

import (
	"encoding/json"
	"gostreambridge/internal/config"
	"log"
	"os"
)

func ConvertConfigFileToMap(configFileName string) map[string]string {
	configMap := make(map[string]string)
	// Read config file
	configData, err := os.ReadFile("internal/config/files/" + configFileName)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	switch configFileName {
	case "elastic.json":
		var currentConfig config.ElasticConfig
		if err := json.Unmarshal(configData, &currentConfig); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"username": currentConfig.Username,
			"password": currentConfig.Password,
			"url":      currentConfig.URL,
			"index":    currentConfig.Index,
		}
	case "mysql.json":
		var currentConfig config.MySQLConfig
		if err := json.Unmarshal(configData, &currentConfig); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"database": currentConfig.Database,
			"username": currentConfig.Username,
			"password": currentConfig.Password,
			"host":     currentConfig.Host,
			"port":     currentConfig.Port,
		}
	case "sqlserver.json":
		var currentConfig config.SQLServerConfig
		if err := json.Unmarshal(configData, &currentConfig); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"database": currentConfig.Database,
			"username": currentConfig.Username,
			"password": currentConfig.Password,
			"host":     currentConfig.Host,
			"port":     currentConfig.Port,
		}
	case "kafka_consumer.json":
	case "kafka_producer.json":
		var currentConfig config.KafkaConfig
		if err := json.Unmarshal(configData, &currentConfig); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"broker":  currentConfig.Broker,
			"topic":   currentConfig.Topic,
			"groupid": currentConfig.GroupId,
		}
	case "rabbitmq.json":
		var currentConfig config.RabbitMQConfig
		if err := json.Unmarshal(configData, &currentConfig); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"url":      currentConfig.URL,
			"username": currentConfig.Username,
			"password": currentConfig.Password,
			"queue":    currentConfig.Queue,
		}
	}
	return configMap
}
