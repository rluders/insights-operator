package kafkarecorder

import (
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/openshift/insights-operator/pkg/record"
	"k8s.io/klog/v2"
)

type KafkaRecorder struct {
	bootstrapSrv string
	topic        string
}

// New diskrecorder driver
func New(bootstrapSrv, topic string) *KafkaRecorder {
	return &KafkaRecorder{bootstrapSrv, topic}
}

// Save the records into the archive
func (d *KafkaRecorder) Save(records record.MemoryRecords) (record.MemoryRecords, error) {
	wrote := 0
	start := time.Now()
	defer func() {
		if wrote > 0 {
			klog.V(2).Infof("Wrote %d records to disk in %s", wrote, time.Since(start).Truncate(time.Millisecond))
		}
	}()

	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": d.bootstrapSrv})
	if err != nil {
		panic(err)
	}

	defer p.Close()

	// Delivery report handler for produced messages
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					klog.V(2).Infof("Delivery failed: %v\n", ev.TopicPartition)
				} else {
					klog.V(2).Infof("Delivered message to %v\n", ev.TopicPartition)
				}
			}
		}
	}()

	completed := make([]record.MemoryRecord, 0, len(records))
	defer func() {
		wrote = len(completed)
	}()

	// Produce messages to topic (asynchronously)
	for _, record := range records {
		p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &d.topic, Partition: kafka.PartitionAny},
			Value:          []byte(record.Name),
			Key:            []byte{},
			Timestamp:      record.At,
			TimestampType:  0,
			Opaque:         nil,
			Headers:        []kafka.Header{},
		}, nil)
		completed = append(completed, record)
	}

	// Wait for message deliveries before shutting down
	p.Flush(15 * 1000)

	return completed, nil
}

// Prune the archives older than given time
func (d *KafkaRecorder) Prune(olderThan time.Time) error {
	return nil
}
