package util

import (
	"log"
	"encoding/json"
	"io/ioutil"
	"gostreambridge/internal/config"
)

func ConvertConfigFileToMap(configFileName string) map[string]string {
	configMap := make(map[string]string)
	// Read config file
	configData, err := ioutil.ReadFile("internal/config/files/" + configFileName)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	if configFileName == "elastic.json" {
		var config config.ElasticConfig
		if err := json.Unmarshal(configData, &config); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"username": config.Username,
			"password": config.Password,
			"url":      config.URL,
			"index":    config.Index,
		}
	}
	if configFileName == "mysql.json" {
		var config config.MySQLConfig
		if err := json.Unmarshal(configData, &config); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"database": config.Database,
			"username": config.Username,
			"password": config.Password,
			"host":     config.Host,
			"port":     config.Port,
		}
	}
	if configFileName == "sqlserver.json" {
		var config config.SQLServerConfig
		if err := json.Unmarshal(configData, &config); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"database": config.Database,
			"username": config.Username,
			"password": config.Password,
			"host":     config.Host,
			"port":     config.Port,
		}
	}
	if configFileName == "kafka_consumer.json" || configFileName == "kafka_producer.json" {
		var config config.KafkaConfig
		if err := json.Unmarshal(configData, &config); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"broker":  config.Broker,
			"topic":   config.Topic,
			"groupid": config.GroupId,
		}
	}
	if configFileName == "rabbitmq.json" {
		var config config.RabbitMQConfig
		if err := json.Unmarshal(configData, &config); err != nil {
			log.Fatalf("Error parsing config JSON: %v", err)
		}
		configMap = map[string]string{
			"url": config.URL,
			"username": config.Username,
			"password": config.Password,
			"queue":    config.Queue,
		}
	}
	return configMap
}