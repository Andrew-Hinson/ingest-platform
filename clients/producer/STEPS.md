# 5d. Go producer

Tiny keyed producer. Not the console CLI. Print `key partition offset` after each ack.

## Done when

```
go run .
```

prints lines like `user-42 1 12`. Forwards still required. No consumer.

## 0. Prereqs

- Three port-forwards still up (`19092`, `19093`, `19094`). If a terminal is idle, restart that forward.
- `nc -vz 127.0.0.1 19092` succeeds.
- `clients/README.md` is `bootstrap: 127.0.0.1:19092`.
- Go installed: `go version`. If missing: `brew install go`.

## 1. Module

From repo root:

```
mkdir -p clients/producer
cd clients/producer
go mod init github.com/Andrew-Hinson/ingest-platform/clients/producer
```

## 2. `main.go`

Create `clients/producer/main.go`:

```go
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
		bootstrap = "127.0.0.1:19092" // clients/README.md
	}

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(bootstrap),
		kgo.DefaultProduceTopic("lab.events"),
		kgo.RequiredAcks(kgo.AllISRAcks()), // acks=all
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
	)
	if err != nil {
		fatal(err)
	}
	defer cl.Close()

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		rec := &kgo.Record{
			Key:   []byte("user-42"),
			Value: []byte(fmt.Sprintf("hello-%d", i)),
		}
		if err := cl.ProduceSync(ctx, rec).FirstErr(); err != nil {
			fatal(err)
		}
		fmt.Printf("%s %d %d\n", string(rec.Key), rec.Partition, rec.Offset)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
```

What this maps to:

| 5d requirement | This code |
|---|---|
| bootstrap from 5c | `127.0.0.1:19092` |
| `acks=all` | `kgo.AllISRAcks()` |
| string key + value | `[]byte("user-42")` / `[]byte(...)` |
| idempotence default | do not set `DisableIdempotentWrite` |
| print key partition offset | `ProduceSync` fills `rec.Partition` / `rec.Offset` |

## 3. Dep + pin

```
go get github.com/twmb/franz-go
go mod tidy
```

Open `go.mod`. Copy the `franz-go` version. Append to `lab/VERSIONS.md`:

```
Go producer: github.com/twmb/franz-go vX.Y.Z
```

## 4. Run

From `clients/producer`, forwards still up:

```
go run .
```

Expect three lines, same partition, rising offsets, e.g.:

```
user-42 1 4
user-42 1 5
user-42 1 6
```

Partition number can be 0, 1, or 2. Same key → same partition is 5e. 5d only needs a printed partition + offset.

## Failures

| Symptom | Cause |
|---|---|
| `connection refused` / dial `127.0.0.1:19092` | port-forward for broker 0 is down |
| timeout, `172.18.0.2` | advertised listeners not applied; re-check `config.yaml` |
| hang then timeout | only one forward up; partition leader is on 19093 or 19094 |

## Stop

Do not start 5e yet. Do not add a consumer. Do not set `acks=1`.

## Steps

1. Confirm three forwards + `nc -vz 127.0.0.1 19092`.
2. `go mod init` in `clients/producer`.
3. Write `main.go` as above.
4. `go get` / `go mod tidy`. Pin version in `lab/VERSIONS.md`.
5. `go run .` until `key partition offset` prints.

**Unresolved:** `franz-go` vs `confluent-kafka-go` (CGO, closer to Java names). Stick with franz-go unless you want librdkafka.
