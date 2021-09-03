package apqm

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Sender struct {
	Address string
}

func (s *Sender) Send(data []byte) (err error) {
	conn, err := amqp.Dial(s.Address)
	if err != nil {
		log.Printf("Error opening connection with AMQP Server %s: %v\n", s.Address, err)
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("Error in connecting to channel %v\n", err)
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"log-messages", // name
		true,           // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		log.Printf("Error declaring Queue %v\n", err)
		return err
	}

	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(data),
		})
	if err != nil {
		log.Printf("Error publishing to Queue %v\n", err)
		return err
	}
	return nil
}
