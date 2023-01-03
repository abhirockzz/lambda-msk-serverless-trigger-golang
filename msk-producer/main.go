package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	sasl_aws "github.com/twmb/franz-go/pkg/sasl/aws"
)

var mskBroker string
var topic string
var client *kgo.Client

func init() {

	mskBroker = os.Getenv("MSK_BROKER")
	if mskBroker == "" {
		log.Fatal("missing env var MSK_BROKER")
	}

	topic = os.Getenv("MSK_TOPIC")
	if mskBroker == "" {
		log.Fatal("missing env var MSK_TOPIC")
	}

	log.Println("MSK_BROKER", mskBroker)
	log.Println("MSK_TOPIC", topic)

	tlsDialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}}

	opts := []kgo.Opt{
		kgo.SeedBrokers(strings.Split(mskBroker, ",")...),
		kgo.SASL(sasl_aws.ManagedStreamingIAM(func(ctx context.Context) (sasl_aws.Auth, error) {

			//log.Println("<<<<< calling auth fn based on iam role >>>>")

			val, err := session.Must(session.NewSession()).Config.Credentials.GetWithContext(ctx)

			if err != nil {
				log.Println("failed to get credentials :( :(", err.Error())
				return sasl_aws.Auth{}, err
			}
			//log.Println("got credentials", val.ProviderName)

			return sasl_aws.Auth{
				AccessKey:    val.AccessKeyID,
				SecretKey:    val.SecretAccessKey,
				SessionToken: val.SessionToken,
				UserAgent:    "franz-go/creds_test/v1.0.0",
			}, nil
		})),
		kgo.Dialer(tlsDialer.DialContext),
	}

	var err error
	client, err = kgo.NewClient(opts...)
	if err != nil {
		log.Fatal(err)
	}

}

func main() {

	fmt.Println("starting producer...")

	var err error

	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	kadmin := kadm.NewClient(client)
	topics, err := kadmin.ListTopics(context.Background(), topic)
	if err != nil {
		log.Println("failed to list topics", err)
	}

	if !topics.Has(topic) {
		resps, err := kadmin.CreateTopics(context.Background(), 3, 2, nil, topic)
		if err != nil {
			log.Fatal("create topic invocation failed", err)
		}

		resp, err := resps.On(topic, nil)
		if err != nil {
			log.Fatal(err)
		}
		if resp.Err != nil {
			log.Fatal(err)
		}

	} else {
		log.Println("topic already exists", topic)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		payload, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("unable to read body")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Println("payload", string(payload))
		defer r.Body.Close()

		log.Println("invoking produce synchronously...")

		res := client.ProduceSync(context.Background(), &kgo.Record{Topic: topic, Value: payload})

		for _, r := range res {
			if r.Err != nil {
				log.Println("produce error:", r.Err)
				http.Error(w, r.Err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Add("kafka-timestamp", r.Record.Timestamp.String())

			log.Println("record produced successfully to offset", r.Record.Offset, "of partition", r.Record.Partition, "in topic", r.Record.Topic)
		}
	})

	log.Println("http server init...")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
