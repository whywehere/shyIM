package kafka

import (
	"context"
	"fmt"
	"github.com/IBM/sarama"
	"shyIM/model"
	"shyIM/pkg/logger"
	"sync"
)

var (
	MQ *Conn
)

func InitMQ(brokers []string) {
	MQ = InitKafka(brokers)
	go MQ.ConsumeMessage()
}

func (c *Conn) ConsumeMessage() {
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := c.consumer.Consume(context.Background(), []string{c.topic}, ConsumerGroupHandlerKafka{}); err != nil {
				logger.Slog.Error("consumer consume failed", "err", err)
				return
			}
		}
	}()
}

// ConsumerGroupHandlerKafka 实现 ConsumerGroupHandler 接口来处理 Kafka 消息
type ConsumerGroupHandlerKafka struct{}

func (h ConsumerGroupHandlerKafka) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h ConsumerGroupHandlerKafka) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h ConsumerGroupHandlerKafka) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		// 处理从 Kafka 接收到的消息
		MessageCreateHandler(msg)

		// 标记消息已经被处理
		session.MarkMessage(msg, "")
	}
	return nil
}

// MessageCreateHandler 用于处理从 Kafka 中接收到的消息
func MessageCreateHandler(msg *sarama.ConsumerMessage) {
	messageModels := model.ProtoMarshalToMessage(msg.Value) // 使用消息的值
	if messageModels == nil {
		// 处理消息为空的情况，这里可以根据实际需求返回不同的处理结果
		return
	}

	err := model.CreateMessage(messageModels...)
	if err != nil {
		fmt.Println("[MessageCreateHandlerKafka] model.CreateMessage 失败，err:", err)
		// 根据业务逻辑返回不同的处理结果，可能是 Nack 或其他操作
		return
	}

	// 根据成功处理后的情况返回 Ack
	return
}
