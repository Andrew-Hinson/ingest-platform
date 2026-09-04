// Kafka consumer group member for lab.events.
// Prints id/partition/offset/key; logs assign/revoke on stderr.
// -commit auto|manual; manual commits after each fetch and on revoke.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/twmb/franz-go/pkg/kgo"
)

const topic = "lab.events"

func main() {
	group := flag.String("group", "lab-events", "consumer group")
	idFlag := flag.String("id", "", "client id")
	commitMode := flag.String("commit", "auto", "auto or manual")
	flag.Parse()

	id := *idFlag
	if id == "" {
		fatal(errors.New("-id is required"))
	}
	if *commitMode != "auto" && *commitMode != "manual" {
		fatal(errors.New("-commit must be auto or manual"))
	}

	bootstrap := os.Getenv("KAFKA_BOOTSTRAP")
	if bootstrap == "" {
		bootstrap = "127.0.0.1:19092"
	}

	manual := *commitMode == "manual"
	opts := []kgo.Opt{
		kgo.SeedBrokers(bootstrap),
		kgo.ConsumerGroup(*group),
		kgo.ConsumeTopics(topic),
		kgo.ClientID(id),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.OnPartitionsAssigned(func(_ context.Context, _ *kgo.Client, assigned map[string][]int32) {
			logParts(id, "assigned", assigned)
		}),
		kgo.OnPartitionsRevoked(func(_ context.Context, cl *kgo.Client, revoked map[string][]int32) {
			logParts(id, "revoked", revoked)
			if !manual {
				return
			}
			if err := commit(context.Background(), cl); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}),
	}
	if manual {
		opts = append(opts, kgo.DisableAutoCommit())
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		fatal(err)
	}
	defer cl.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Keep polling until SIGINT/SIGTERM or the client closes.
	// Print each record. On a fetch error, log it and poll again.
	// In manual mode, commit after a non-empty batch; a failed commit exits.
	for {
		fetches := cl.PollFetches(ctx)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, e.Err)
			}
			continue
		}
		fetches.EachRecord(func(r *kgo.Record) {
			fmt.Printf("%s %d %d %s\n", id, r.Partition, r.Offset, string(r.Key))
		})
		if manual && fetches.NumRecords() > 0 {
			if err := commit(ctx, cl); err != nil {
				fatal(err)
			}
		}
	}
}

func commit(ctx context.Context, cl *kgo.Client) error {
	return cl.CommitUncommittedOffsets(ctx)
}

func logParts(id, verb string, parts map[string][]int32) {
	fmt.Fprintf(os.Stderr, "%s %s %s %v\n", id, verb, topic, parts[topic])
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
