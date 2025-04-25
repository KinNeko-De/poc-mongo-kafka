package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run kafka_test.go [produce|consume]")
		os.Exit(1)
	}

	// Kafka connection settings
	brokers := []string{"localhost:9095"}
	topic := "test"

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		fmt.Println("\nShutting down...")
		cancel()
	}()

	switch os.Args[1] {
	case "produce":
		produceMessages(ctx, brokers, topic)
	case "consume":
		consumeMessages(ctx, brokers, topic)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		fmt.Println("Usage: go run kafka_test.go [produce|consume]")
		os.Exit(1)
	}
}

func produceMessages(ctx context.Context, brokers []string, topic string) {
	// Create a writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  brokers,
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	fmt.Println("Press Ctrl+C to stop producing messages")
	fmt.Println("Producing messages to topic:", topic)

	count := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			message := fmt.Sprintf("Message %d from Go producer at %s", count, time.Now().Format(time.RFC3339))

			// Write the message
			err := writer.WriteMessages(ctx, kafka.Message{
				Value: []byte(message),
			})
			if err != nil {
				log.Printf("Error writing message: %v", err)
			} else {
				fmt.Printf("Produced: %s\n", message)
				count++
			}

			time.Sleep(2 * time.Second)
		}
	}
}

func consumeMessages(ctx context.Context, brokers []string, topic string) {
	// Create a reader with explicit timeout
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		MaxWait:  1 * time.Second,
		// GroupID:        "go-kafka-test-group-" + time.Now().Format("20060102150405"), // Use a unique group ID
		StartOffset:    kafka.FirstOffset,
		SessionTimeout: 30 * time.Second,
		ReadBackoffMin: 100 * time.Millisecond,
		ReadBackoffMax: 1 * time.Second,
	})
	defer reader.Close()

	fmt.Println("Press Ctrl+C to stop consuming messages")
	fmt.Println("Consuming messages from topic:", topic)
	fmt.Printf("Connected to brokers: %v\n", brokers)

	// Check if topic exists first
	conn, err := kafka.DialLeader(ctx, "tcp", brokers[0], topic, 0)
	if err != nil {
		fmt.Printf("Failed to connect to leader: %v\n", err)
		return
	}
	conn.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			message, err := reader.ReadMessage(ctx)
			if err != nil {
				if err == io.EOF {
					log.Printf("Reached end of partition, waiting for new messages...")
					time.Sleep(2 * time.Second)
					continue
				}
				log.Printf("Error reading message: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			fmt.Printf("Consumed message at offset %d: %s\n", message.Offset, string(message.Value))
		}
	}
}
