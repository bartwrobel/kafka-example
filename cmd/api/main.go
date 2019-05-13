package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bartwrobel/kafka-example/platform/kafka"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

const (
	listenAddrApi = "localhost:9000"

	// kafka
	kafkaBrokerUrl = "192.168.99.100:19091,192.168.99.100:29091,192.168.99.100:39092"
	kafkaClientId  = "consumer-group-id"
	kafkaTopic     = "orders-new"
)

func main() {

	logger.Out = os.Stdout

	// connect to kafka
	kafkaProducer, err := kafka.Configure(strings.Split(kafkaBrokerUrl, ","), kafkaClientId, kafkaTopic)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("unable to configure kafka")
	}
	defer kafkaProducer.Close()
	var errChan = make(chan error, 1)

	go func() {
		logger.Infof("starting server at %s", listenAddrApi)
		errChan <- server(listenAddrApi)
	}()

	var signalChan = make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	select {
	case <-signalChan:
		logger.Info("got an interrupt, exiting...")
	case err := <-errChan:
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err.Error()}).Error("error while running api, exiting...")
		}
	}
}

func server(listenAddr string) error {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.POST("/api/v1/order", processOrder)

	// for debugging purpose
	for _, routeInfo := range router.Routes() {
		logger.WithFields(logrus.Fields{
			"path":    routeInfo.Path,
			"handler": routeInfo.Handler,
			"method":  routeInfo.Method,
		}).Info("registered routes")
	}

	return router.Run(listenAddr)
}

func processOrder(c *gin.Context) {
	ctx := context.Background()
	defer ctx.Done()

	type NewOrderRequest struct {
		Price    int `json:"price"`
		Customer struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
		} `json:"customer"`
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

	var body = &NewOrderRequest{}
	if err := c.ShouldBindBodyWith(&body, binding.JSON); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("error while binding request")
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("error while processing new order request: %s", err.Error()),
			},
		})
		return
	}

	newOrderCmd := NewOrderCmd{
		OrderId:   uuid.New().String(),
		Price:     body.Price,
		Customer:  body.Customer,
		Status:    "new",
		CreatedAt: time.Now(),
	}

	cmdMarshaled, err := json.Marshal(newOrderCmd)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("error while marshaling new order command")
		c.JSON(http.StatusUnprocessableEntity, map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("error while marshaling new order command: %s", err.Error()),
			},
		})
		return
	}

	if err := kafka.Push(ctx, []byte(newOrderCmd.OrderId), cmdMarshaled); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("error while running api, exiting...")
		c.JSON(http.StatusUnprocessableEntity, map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("error while push message into kafka: %s", err.Error()),
			},
		})
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "success push data into kafka",
		"data":    newOrderCmd,
	})
}
