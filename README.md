# go-stream-bridge

- Application to read from an upstream source (Kafka / RabbitMQ) and then provide processing capabilities to the user and write to a downstream source (Kafka / MySQL / SQL Server / Elastic)
- The processing part is left to the user which can be done at a case-by-case basis
- This provides a platform where the heavylifting of consumption and production can be handled by the codebase in a modular fashion, while the user can focus on the main code which would process the message.

### Basic Functionalities:

1. **Connect to Messaging Queue**:
   - Establish connections to one or more messaging queues.
   - Support for popular messaging protocols such as AMQP, Kafka, etc.

2. **Consume Messages**:
   - Listen for incoming messages from the subscribed queue(s).
   - Handle message consumption in a concurrent and efficient manner.

3. **Logging and Monitoring**:
   - Logging of important events, errors, and informational messages.

4. **Data Persistence**:
   - Write processed data to a downstream datastore (e.g., database, cache, file system, messaging queues).
   - Ensure data integrity and reliability during the persistence process.

5. **Concurrency Management**:
   - Efficiently handle concurrent message processing using goroutines and channels.

6. **Configuration**:
   - Allow configuration through configuration files and command-line arguments.

7. **Graceful Shutdown**:
    - Enable graceful shutdown procedures to ensure clean termination of the application.

### Running the code

Execute the following steps to run the code:

1. `cd` to the root directory.
2. Update the `config/` directory in `internal/` directory with the right configurations for your application.
3. Run `go build .`
4. Run `go run main.go -upstreamApp=valueX -downstreamApp=valueY` and replace `valueX` and `valueY` with the appropriate values.

### Kafka Upstream and Downstream Run steps

1. **Download Kafka:**
   - Go to the Kafka website and download the latest stable release.

2. **Extract and Navigate:**
   - Extract the downloaded Kafka archive and navigate to the Kafka directory.

3. **Start Zookeeper, kafka, and create topics:**
   - Start Zookeeper server:
     ```
     bin/zookeeper-server-start.sh config/zookeeper.properties
     ```
   - Start Kafka server:
     ```
     bin/kafka-server-start.sh config/server.properties
     ```
   - Create test topics:
     ```
     bin/kafka-topics.sh --create --zookeeper localhost:2181 --replication-factor 1 --partitions 1 --topic topic1
     bin/kafka-topics.sh --create --zookeeper localhost:2181 --replication-factor 1 --partitions 1 --topic topic2
     ```

4. **Send and Listen Messages:**
   - Send a message to `topic1`:
     ```
     bin/kafka-console-producer.sh --broker-list localhost:9092 --topic topic1
     > Hello, Kafka!
     ```
   - Listen for messages on `topic2`:
     ```
     bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic topic2 --from-beginning
     ```

5. **Run the Go code:**
   - Update the `kafka_consumer.json` and `kafka_producer.json` with your topic and server details. Use the below command to run the code:
     ```
     go run main.go -upstreamApp=kafka -downstreamApp=kafka
     ```
     
6. **Verify:**
   - Verify that the message is consumed from `topic1` and produced to `topic2`.
