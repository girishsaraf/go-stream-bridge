// Package connectors registers all first-party bridge connector factories.
package connectors

import (
	_ "gostreambridge/connectors/elasticsearch"
	_ "gostreambridge/connectors/kafka"
	_ "gostreambridge/connectors/mysql"
	_ "gostreambridge/connectors/rabbitmq"
	_ "gostreambridge/connectors/sqlserver"
)
