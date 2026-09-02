package main

import (
	"context"
	"fmt"
	"os"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	bootstrap := os.Getenv("KAFKA_BOOTSTRAP")
	if bootstrap == "" {
		bootstrap = "127.0.0.1:19092"
	}

	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(bootstrap),
		kgo.DefaultProduceTopic("lab.events"),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
	)
	if err != nil {
		fatal(err)
	}
	defer kafkaClient.Close()

	ctx := context.Background()
	for i := range 20 {
		send(ctx, kafkaClient, "user-42", fmt.Appendf(nil, "same-%d", i))
	}
	for i := range 10 {
		send(ctx, kafkaClient, fmt.Sprintf("user-%d", i), fmt.Appendf(nil, "diff-%d", i))
	}
}

func send(ctx context.Context, cl *kgo.Client, key string, value []byte) {
	rec := &kgo.Record{Key: []byte(key), Value: value}
	if err := cl.ProduceSync(ctx, rec).FirstErr(); err != nil {
		fatal(err)
	}
	fmt.Printf("%s %d %d\n", string(rec.Key), rec.Partition, rec.Offset)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
