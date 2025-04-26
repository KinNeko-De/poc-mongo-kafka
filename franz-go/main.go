package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go [produce|consume]")
		os.Exit(1)
	}

	// Kafka connection settings
	brokers := []string{"localhost:9095"}
	topic := "test-franz"

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
		fmt.Println("Usage: go run main.go [produce|consume]")
		os.Exit(1)
	}
}

func produceMessages(ctx context.Context, brokers []string, topic string) {
	// Create a producer client
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.WithLogger(kgo.BasicLogger(os.Stderr, kgo.LogLevelInfo, nil)),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Press Ctrl+C to stop producing messages")
	fmt.Println("Producing messages to topic:", topic)

	count := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			message := fmt.Sprintf("Message %d from Go producer at %s", count, time.Now().Format(time.RFC3339))

			// Create and produce the record
			record := &kgo.Record{
				Topic: topic,
				Value: []byte(message),
			}

			err := client.ProduceSync(ctx, record).FirstErr()
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
	// Create a consumer client
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()), // Start from beginning
		kgo.WithLogger(kgo.BasicLogger(os.Stderr, kgo.LogLevelInfo, nil)),
		// Use a unique group ID to read from beginning each time
		kgo.ConsumerGroup(fmt.Sprintf("go-kafka-test-group-%s", time.Now().Format("20060102150405"))),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Press Ctrl+C to stop consuming messages")
	fmt.Println("Consuming messages from topic:", topic)
	fmt.Printf("Connected to brokers: %v\n", brokers)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Poll for messages
			fetches := client.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return
			}

			// Check for errors
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					log.Printf("Error consuming from topic %s: %v", err.Topic, err.Err)
				}
				time.Sleep(1 * time.Second)
				continue
			}

			// Process fetched records
			fetches.EachPartition(func(p kgo.FetchTopicPartition) {
				for _, record := range p.Records {
					fmt.Printf("Consumed message at offset %d: %s\n",
						record.Offset, string(record.Value))
				}
			})

			// If no messages were fetched, wait a bit before polling again
			if fetches.NumRecords() == 0 {
				time.Sleep(1 * time.Second)
			}
		}
	}
}
