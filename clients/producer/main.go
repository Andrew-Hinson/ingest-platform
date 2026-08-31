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
	for i := range 3 {
		record := &kgo.Record{
			Key:   []byte("user-42"),
			Value: fmt.Appendf(nil, "hello-%d", i),
		}
		if err := kafkaClient.ProduceSync(ctx, record).FirstErr(); err != nil {
			fatal(err)
		}
		fmt.Printf("%s %d %d\n", string(record.Key), record.Partition, record.Offset)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
