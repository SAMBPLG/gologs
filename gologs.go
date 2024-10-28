package gologs

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	AuditTopicName    = "audit_logs"
	ActivityTopicName = "activity_logs"
)

// AuditLog represents a single audit log entry.
type AuditLog struct {
	Module     string    `json:"module"`
	ActionType string    `json:"actionType"`
	SearchKey  string    `json:"searchKey"`
	Before     string    `json:"before"`
	After      string    `json:"after"`
	ActionBy   string    `json:"actionBy"`
	ActionTime time.Time `json:"timestamp"`
}

// ActivityLog represents a single activity log entry.
type ActivityLog struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Activity     string    `json:"activity"`
	Module       string    `json:"module"`
	ActivityTime time.Time `json:"activity_time"`
	IPAddress    string    `json:"ip_address,omitempty"`
	DeviceInfo   string    `json:"device_info,omitempty"`
	Location     string    `json:"location,omitempty"`
	Remarks      string    `json:"remarks,omitempty"`
}

// LogClient manages the connection and channel for RabbitMQ.
type LogClient struct {
	connection *amqp.Connection
	channel    *amqp.Channel
}

// Global instance of LogClient
var logClient *LogClient

// InitAuditLogClient initializes the global AuditLogClient.
func InitAuditLogClient() error {
	return initLogClient(AuditTopicName)
}

// InitActivityLogClient initializes the global ActivityLogClient.
func InitActivityLogClient() error {
	return initLogClient(ActivityTopicName)
}

// initLogClient initializes the log client and declares the queue.
func initLogClient(topicName string) error {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Could not load .env file, using environment variables from the host")
	}

	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if rabbitMQURL == "" {
		return errors.New("RABBITMQ_URL must be set in the environment variables or .env file")
	}

	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	if _, err := ch.QueueDeclare(
		topicName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("failed to declare a queue: %w", err)
	}

	logClient = &LogClient{
		connection: conn,
		channel:    ch,
	}

	return nil
}

// PublishAuditLog sends an audit log to the RabbitMQ queue.
func PublishAuditLog(log AuditLog) error {
	log.ActionTime = time.Now()
	payload, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal audit log: %w", err)
	}

	if err := logClient.channel.Publish(
		"",             // exchange
		AuditTopicName, // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
		},
	); err != nil {
		return fmt.Errorf("failed to publish audit message: %w", err)
	}

	return nil
}

// PublishActivityLog sends an activity log to the RabbitMQ queue.
func PublishActivityLog(log ActivityLog) error {
	log.ActivityTime = time.Now()
	payload, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal activity log: %w", err)
	}

	if err := logClient.channel.Publish(
		"",                // exchange
		ActivityTopicName, // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
		},
	); err != nil {
		return fmt.Errorf("failed to publish activity message: %w", err)
	}

	return nil
}

// ConsumeAuditLogs starts consuming audit logs from the queue.
func ConsumeAuditLogs(consumerName *string, handler func(AuditLog, func(bool)), prefetchCount *int) error {
	if consumerName == nil {
		defaultName := "default_audit_consumer"
		consumerName = &defaultName
	}

	var effectivePrefetchCount int
	if prefetchCount != nil {
		effectivePrefetchCount = *prefetchCount
	} else {
		effectivePrefetchCount = 50 // Default value
	}

	if err := logClient.channel.Qos(effectivePrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := logClient.channel.Consume(
		AuditTopicName, // queue
		*consumerName,  // consumer name
		false,          // auto-ack
		false,          // exclusive
		false,          // no-local
		false,          // no-wait
		nil,            // args
	)
	if err != nil {
		return fmt.Errorf("failed to register an audit consumer: %w", err)
	}

	go func() {
		for msg := range msgs {
			var auditLog AuditLog
			if err := json.Unmarshal(msg.Body, &auditLog); err != nil {
				log.Printf("Error unmarshaling audit message: %v", err)
				continue
			}

			handler(auditLog, func(ack bool) {
				if ack {
					if err := msg.Ack(false); err != nil {
						log.Printf("Failed to acknowledge audit message: %v", err)
					}
				} else {
					if err := msg.Nack(false, true); err != nil {
						log.Printf("Failed to nack audit message: %v", err)
					}
				}
			})
		}
	}()

	log.Printf("Audit consumer %s is waiting for messages. To exit press CTRL+C", *consumerName)
	return nil
}

// ConsumeActivityLogs starts consuming activity logs from the queue.
func ConsumeActivityLogs(consumerName *string, handler func(ActivityLog, func(bool)), prefetchCount *int) error {
	if consumerName == nil {
		defaultName := "default_activity_consumer"
		consumerName = &defaultName
	}

	var effectivePrefetchCount int
	if prefetchCount != nil {
		effectivePrefetchCount = *prefetchCount
	} else {
		effectivePrefetchCount = 50 // Default value
	}

	if err := logClient.channel.Qos(effectivePrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := logClient.channel.Consume(
		ActivityTopicName, // queue
		*consumerName,     // consumer name
		false,             // auto-ack
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	if err != nil {
		return fmt.Errorf("failed to register an activity consumer: %w", err)
	}

	go func() {
		for msg := range msgs {
			var activityLog ActivityLog
			if err := json.Unmarshal(msg.Body, &activityLog); err != nil {
				log.Printf("Error unmarshaling activity message: %v", err)
				continue
			}

			handler(activityLog, func(ack bool) {
				if ack {
					if err := msg.Ack(false); err != nil {
						log.Printf("Failed to acknowledge activity message: %v", err)
					}
				} else {
					if err := msg.Nack(false, true); err != nil {
						log.Printf("Failed to nack activity message: %v", err)
					}
				}
			})
		}
	}()

	log.Printf("Activity consumer %s is waiting for messages. To exit press CTRL+C", *consumerName)
	return nil
}

// Close closes the channel and connection of the LogClient.
func Close() {
	if logClient.channel != nil {
		_ = logClient.channel.Close()
	}
	if logClient.connection != nil {
		_ = logClient.connection.Close()
	}
}

// CloseGlobalClient closes the global log client.
func CloseGlobalClient() {
	Close()
}