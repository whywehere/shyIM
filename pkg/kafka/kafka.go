package kafka

import (
	"fmt"
	"github.com/IBM/sarama"
	"shyIM/pkg/logger"
)

const (
	MessageGroupID = "message.group"
	MessageTopic   = "message.topic"
)

type Conn struct {
	producer sarama.SyncProducer
	consumer sarama.ConsumerGroup
	topic    string
	groupID  string
}

func InitKafka(brokers []string) *Conn {
	config := sarama.NewConfig()
	config.Producer.Return.Errors = true    // 设定是否需要返回错误信息
	config.Producer.Return.Successes = true // 设定是否需要返回成功信息
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		logger.Slog.Error("Failed to create Kafka producer", "err", err)
		return nil
	}

	consumer, err := sarama.NewConsumerGroup(brokers, MessageGroupID, config)
	if err != nil {
		logger.Slog.Error("Failed to create Kafka consumerGroup", "err", err)
		return nil
	}

	return &Conn{
		producer: producer,
		consumer: consumer,
		topic:    MessageTopic,
		groupID:  MessageGroupID,
	}
}

func (c *Conn) Publish(data []byte) error {
	if data == nil || len(data) == 0 {
		fmt.Println("Failed to publish: data is empty")
		return nil
	}
	msg := &sarama.ProducerMessage{
		Topic: c.topic,
		Value: sarama.ByteEncoder(data),
	}
	_, _, err := c.producer.SendMessage(msg)
	return err
}
func (c *Conn) Close() {
	c.producer.Close()
	c.consumer.Close()
}
