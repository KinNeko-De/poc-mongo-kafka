package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go [produce|consume]")
		os.Exit(1)
	}

	// Kafka connection settings
	brokers := []string{"localhost:9095"}
	topic := "test-samara"

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
	// Configure producer
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	// Create sync producer
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.Printf("Error closing producer: %v", err)
		}
	}()

	fmt.Println("Press Ctrl+C to stop producing messages")
	fmt.Println("Producing messages to topic:", topic)

	count := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			message := fmt.Sprintf("Message %d from Go producer at %s", count, time.Now().Format(time.RFC3339))

			// Create the message
			msg := &sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.StringEncoder(message),
			}

			// Send the message
			partition, offset, err := producer.SendMessage(msg)
			if err != nil {
				log.Printf("Error writing message: %v", err)
			} else {
				fmt.Printf("Produced message to partition %d at offset %d: %s\n",
					partition, offset, message)
				count++
			}

			time.Sleep(2 * time.Second)
		}
	}
}

func consumeMessages(ctx context.Context, brokers []string, topic string) {
	// Configure consumer
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetOldest // Start from oldest message
	config.Version = sarama.V2_0_0_0                      // Compatible with most Kafka versions

	// Create a unique consumer group ID each time
	groupID := fmt.Sprintf("go-kafka-test-group-%s", time.Now().Format("20060102150405"))

	// Create consumer group
	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		log.Fatalf("Error creating consumer group: %v", err)
	}
	defer func() {
		if err := consumerGroup.Close(); err != nil {
			log.Printf("Error closing consumer group: %v", err)
		}
	}()

	// Handle consumer errors
	go func() {
		for err := range consumerGroup.Errors() {
			log.Printf("Consumer group error: %v", err)
		}
	}()

	fmt.Println("Press Ctrl+C to stop consuming messages")
	fmt.Println("Consuming messages from topic:", topic)
	fmt.Printf("Connected to brokers: %v with group: %s\n", brokers, groupID)

	// Create consumer handler
	handler := &ConsumerGroupHandler{
		ready: make(chan bool),
	}

	// Consume in a loop
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			// Check if context is cancelled
			if ctx.Err() != nil {
				return
			}

			// Consume from the topic
			err := consumerGroup.Consume(ctx, []string{topic}, handler)
			if err != nil {
				log.Printf("Error from consumer: %v", err)
			}

			// Check if context is cancelled
			if ctx.Err() != nil {
				return
			}

			// Mark the consumer as ready for the next session
			handler.ready = make(chan bool)
		}
	}()

	// Wait for consumer to be ready
	<-handler.ready
	log.Println("Consumer is ready")

	// Wait for cancellation
	<-ctx.Done()
	log.Println("Shutting down consumer")
	wg.Wait()
}

// ConsumerGroupHandler implements the ConsumerGroupHandler interface
type ConsumerGroupHandler struct {
	ready chan bool
}

// Setup is run at the beginning of a new session
func (h *ConsumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	close(h.ready) // Mark the consumer as ready
	return nil
}

// Cleanup is run at the end of a session
func (h *ConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim is called to consume messages
func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// Read messages from the claim
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil // Channel closed
			}
			// Process the message
			fmt.Printf("Consumed message at partition %d, offset %d: %s\n",
				message.Partition, message.Offset, string(message.Value))

			// Mark the message as consumed
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil // Context cancelled
		}
	}
}
