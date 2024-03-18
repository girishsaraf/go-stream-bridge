package config

// Configuration struct for MySQL database
type MySQLConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
}

// Configuration struct for Elasticsearch
type ElasticConfig struct {
	URL      string `json:"url"` // URL with port
	Username string `json:"username"`
	Password string `json:"password"`
}

// Configuration struct for SQL Server
type SQLServerConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
}

// Configuration struct for Kafka
type KafkaConfig struct {
	Broker  string `json:"brokers"` // Broker with port
	Topic   string `json:"topic"`
	GroupId string `json:"groupid"`
}

// Configuration struct for RabbitMQ
type RabbitMQConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Queue    string `json:"queue"`
}