package util

import (
	"encoding/json"
	"fmt"
	"gostreambridge/internal/config"
	"os"
	"path/filepath"
)

// ConvertConfigFileToMap reads a configuration file and converts it to a map
func ConvertConfigFileToMap(configFileName string) (map[string]string, error) {
	configMap := make(map[string]string)

	// Read config file
	configData, err := os.ReadFile(filepath.Join("internal", "config", "files", configFileName))
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Unmarshal JSON based on config file type
	switch configFileName {
	case "elastic.json":
		var currentConfig config.ElasticConfig
		if err := json.Unmarshal(configData, &currentConfig); err != nil {
			return nil, fmt.Errorf("error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"username": currentConfig.Username,
			"password": currentConfig.Password,
			"url":      currentConfig.URL,
			"index":    currentConfig.Index,
		}
	case "mysql.json", "sqlserver.json":
		var currentConfig config.DBConfig
		if err := json.Unmarshal(configData, &currentConfig); err != nil {
			return nil, fmt.Errorf("error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"database": currentConfig.Database,
			"username": currentConfig.Username,
			"password": currentConfig.Password,
			"host":     currentConfig.Host,
			"port":     currentConfig.Port,
		}
	case "kafka_consumer.json", "kafka_producer.json":
		var currentConfig config.KafkaConfig
		if err := json.Unmarshal(configData, &currentConfig); err != nil {
			return nil, fmt.Errorf("error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"broker":  currentConfig.Broker,
			"topic":   currentConfig.Topic,
			"groupId": currentConfig.GroupId,
		}
	case "rabbitmq.json":
		var currentConfig config.RabbitMQConfig
		if err := json.Unmarshal(configData, &currentConfig); err != nil {
			return nil, fmt.Errorf("error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"url":      currentConfig.URL,
			"username": currentConfig.Username,
			"password": currentConfig.Password,
			"queue":    currentConfig.Queue,
		}
	default:
		return nil, fmt.Errorf("unsupported config file type: %s", configFileName)
	}

	return configMap, nil
}
