package main

import (
	"context"
	"encoding/base64"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, evt events.KafkaEvent) {

	for _, r := range evt.Records {
		for _, kr := range r {

			log.Println("event from offset", kr.Offset, "in partition", kr.Partition, "for topic", kr.Topic)

			val, err := base64.StdEncoding.DecodeString(kr.Value)
			if err != nil {
				log.Println("value base64 decoding failed", err)
			}
			log.Println("received message -", string(val))

			if kr.Key != "" {
				log.Println("record key base64", kr.Key)

				key, err := base64.StdEncoding.DecodeString(kr.Key)
				if err != nil {
					log.Println("key base64 decoding failed", err)
				}
				log.Println("key", string(key))
			}
		}
	}
}
