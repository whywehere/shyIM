package mq

import (
	"fmt"
	"github.com/wagslane/go-rabbitmq"
	"shyIM/model"
)

const (
	MessageQueue        = "message.queue"
	MessageRoutingKey   = "message.routing.key"
	MessageExchangeName = "message.exchange.name"
)

var (
	MessageMQ *Conn
)

func InitMessageMQ(url string) {
	MessageMQ = InitRabbitMQ(url, MessageCreateHandler, MessageQueue, MessageRoutingKey, MessageExchangeName)
}

func MessageCreateHandler(d rabbitmq.Delivery) rabbitmq.Action {
	messageModels := model.ProtoMarshalToMessage(d.Body)
	if messageModels == nil {
		return rabbitmq.NackDiscard
	}
	err := model.CreateMessage(messageModels...)
	if err != nil {
		fmt.Println("[MessageCreateHandler] model.CreateMessage 失败，err:", err)
		return rabbitmq.NackDiscard
	}

	return rabbitmq.Ack
}
