package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"encoding/json"

	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

const (
	// kafka
	kafkaBrokerUrl = "192.168.99.100:19091,192.168.99.100:29091,192.168.99.100:39092"
	kafkaClientId  = "consumer-group-id"
	kafkaTopic     = "orders-new"
)

var logger = logrus.New()

func main() {

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	brokers := strings.Split(kafkaBrokerUrl, ",")

	config := kafka.ReaderConfig{
		Brokers:         brokers,
		GroupID:         kafkaClientId,
		Topic:           kafkaTopic,
		MinBytes:        10e3,            // 10KB
		MaxBytes:        10e6,            // 10MB
		MaxWait:         1 * time.Second, // Maximum amount of time to wait for new data to come when fetching batches of messages from kafka.
		ReadLagInterval: -1,
	}

	reader := kafka.NewReader(config)
	defer reader.Close()

	fmt.Println("start consuming ... !!")

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			logger.WithError(err).Error("error while reading message")
			continue
		}

		type NewOrderCmd struct {
			OrderId  string `json:"order_id"`
			Price    int    `json:"price"`
			Customer struct {
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				Email     string `json:"email"`
			} `json:"customer"`
			Status    string    `json:"status"`
			CreatedAt time.Time `json:"created_at"`
		}

		var newOrderCmd NewOrderCmd
		if err := json.Unmarshal(m.Value, &newOrderCmd); err != nil {
			logger.WithError(err).Error("error while unmarshal message")
			continue
		}

		fmt.Printf("partition[offset]: %d[%d]: %s = %+v\n", m.Partition, m.Offset, string(m.Key), newOrderCmd)
	}
}
