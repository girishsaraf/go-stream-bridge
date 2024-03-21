package config

// DBConfig Configuration struct for databases
type DBConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
}

// MySQLConfig Configuration struct for MySQL
type MySQLConfig struct {
	DBConfig
}

// SQLServerConfig Configuration struct for SQL Server
type SQLServerConfig struct {
	DBConfig
}

// CommonDBConfig defines common configuration fields
type CommonDBConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ElasticConfig Configuration struct for Elastic
type ElasticConfig struct {
	CommonDBConfig
	Index string `json:"index"`
}

// RabbitMQConfig Configuration struct for RabbitMQ
type RabbitMQConfig struct {
	CommonDBConfig
	Queue string `json:"queue"`
}

// KafkaConfig Configuration struct for Kafka
type KafkaConfig struct {
	Broker  string `json:"brokers"` // Broker with port
	Topic   string `json:"topic"`
	GroupId string `json:"groupid"`
}
