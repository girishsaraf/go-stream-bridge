package config

// MySQLConfig Configuration struct for MySQL database
type MySQLConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
}

// ElasticConfig Configuration struct for Elasticsearch
type ElasticConfig struct {
	URL      string `json:"url"` // URL with port
	Username string `json:"username"`
	Password string `json:"password"`
	Index    string `json:"index"`
}

// SQLServerConfig Configuration struct for SQL Server
type SQLServerConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
}

// KafkaConfig Configuration struct for Kafka
type KafkaConfig struct {
	Broker  string `json:"brokers"` // Broker with port
	Topic   string `json:"topic"`
	GroupId string `json:"groupid"`
}

// RabbitMQConfig Configuration struct for RabbitMQ
type RabbitMQConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Queue    string `json:"queue"`
}
