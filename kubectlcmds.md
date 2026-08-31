# Kubectl Commons

### Find a broker

```kubectl -n kafka get pods```

### Exec into broker

```kubectl -n kafka exec -it ingest-platform-dual-role-0 -c kafka -- bash```

### Produce from inside pod
```
/opt/kafka/bin/kafka-console-producer.sh --bootstrap-server localhost:9092 \
  --topic lab.events \
  --reader-property parse.key=true \
  --reader-property key.separator=:
```

### Input dummy data
`user-42:hello`

