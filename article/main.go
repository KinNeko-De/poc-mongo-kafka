package main

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	topic := "test-v2"
	partition := 0
	conn, _ := connect(topic, partition)

	// writeMessages(conn, []string{"msg 1", "msg 22", "msg 333"})
	// readMessages(conn, 10, 10e3)
	readWithReader(topic, "consumer-through-kafka 1")

	if err := conn.Close(); err != nil {
		fmt.Println("failed to close connection:", err)
	}
} //end main

// Connect to the specified topic and partition in the server
func connect(topic string, partition int) (*kafka.Conn, error) {
	conn, err := kafka.DialLeader(context.Background(), "tcp",
		"localhost:9095", topic, partition)
	if err != nil {
		fmt.Println("failed to dial leader")
	}
	return conn, err
} //end connect

// Writes the messages in the string slice to the topic
func writeMessages(conn *kafka.Conn, msgs []string) {
	var err error
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	for _, msg := range msgs {
		_, err = conn.WriteMessages(
			kafka.Message{Value: []byte(msg)})
		fmt.Printf("wrote %s\n", msg)
	}
	if err != nil {
		fmt.Println("failed to write messages:", err)
	}
} //end writeMessages

// Reads all messages in the partition from the start
// Specify a minimum and maximum size in bytes to read (1 char = 1 byte)
func readMessages(conn *kafka.Conn, minSize int, maxSize int) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	batch := conn.ReadBatch(minSize, maxSize) //in bytes

	msg := make([]byte, 10e3) //set the max length of each message
	for {
		msgSize, err := batch.Read(msg)
		if err != nil {
			break
		}
		fmt.Println(string(msg[:msgSize]))
	}

	if err := batch.Close(); err != nil { //make sure to close the batch
		fmt.Println("failed to close batch:", err)
	}
} //end readMessages

// Read from the topic using kafka.Reader
// Readers can use consumer groups (but are not required to)
func readWithReader(topic string, groupID string) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"localhost:9095"},
		// GroupID:        groupID,
		Topic:          topic,
		MaxBytes:       10e6,              // Increased to handle larger messages
		StartOffset:    kafka.FirstOffset, // Start from beginning of the topic
		MinBytes:       10,                // Minimum number of bytes to fetch
		ReadBackoffMin: 100 * time.Millisecond,
		ReadBackoffMax: 1 * time.Second,
	})
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("Reading from topic %s with group %s\n", topic, groupID)

	// Try to fetch messages
	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			if err == context.DeadlineExceeded {
				fmt.Println("Timeout reached, no more messages")
			} else {
				fmt.Printf("Failed to read message: %v\n", err)
			}
			break
		}
		fmt.Printf("Message at topic/partition/offset %v/%v/%v: %s\n",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Value))
	}
}
