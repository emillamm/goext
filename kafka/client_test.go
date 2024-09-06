package kafka

import (
	"testing"
	"os"
	"math/rand"
	"context"
	"sync"
	"fmt"
	"time"
	"errors"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestKafkaClient(t *testing.T) {

	ctx := context.Background()

	var client *KafkaClient
	defer func() {
		if client != nil && !client.IsClosed() {
			client.Close()
			client.WaitForDone()
		}
	}()

	t.Run("", func(t *testing.T) {
	})

	t.Run("publish to topic without retry and dql", func(t *testing.T) {

		// Initialize values
		topic := "test-topic-1"
		group := randomGroup()
		startFrom := time.Now()

		// Ensure that we start consuming from the right timestamp
		env := func(s string) string {
			switch s {
			case "KAFKA_CONSUMER_START_FROM":
				return startFrom.Format(time.RFC3339)
			default:
				return os.Getenv(s)
			}
		}

		// Create client
		ctx, _ = context.WithTimeout(ctx, 10 * time.Second) // client timeout
		client, err := NewKafkaClient(ctx, env)
		if err != nil { t.Fatal(err) }

		// Create record verifier
		ctx, _ = context.WithTimeout(ctx, 7 * time.Second) // verifier timeout
		verifier := newRecordVerifier[string](ctx, "r1")

		// Set group
		client.SetGroup(group)

		// register consumer without retry and dlq
		err = client.RegisterConsumer(topic, 0, false, func(record *ConsumeRecord) {
			if err := verifier.receive(string(record.Underlying.Value)); err != nil {
				t.Error(err)
			}
			record.Ack()
		})
		if err != nil {
			t.Fatal(err)
		}

		// start client and handle errors
		errs := client.Start()
		go func() {
			for {
				select {
				case err := <-errs:
					if errors.Is(err, ErrClientClosed) {
						return
					}
					t.Error(err)
				}
			}
		}()

		// Check that client is started
		if !client.IsStarted() {
			t.Fatal("client is not started")
		}

		// publish expected record
		record := &kgo.Record{
			Value: []byte("r1"),
			Topic: topic,
		}
		if err = client.PublishRecord(ctx, topic, record); err != nil {
			t.Fatal(err)
		}

		// Check and verify consumed records
		select {
		case <-verifier.Done():
			if err := verifier.Err(); err != nil {
				t.Fatal(err)
			}
		}

		// Close client
		client.CloseGracefylly(ctx)
		client.WaitForDone()
		time.Sleep(10 * time.Millisecond) // allow ErrClientClosed to arrive
	})

	t.Run("publish to topic with retry and dql", func(t *testing.T) {
	})

	t.Run("return errors", func(t *testing.T) {
	})

	t.Run("IsStarted should be true after client is started", func(t *testing.T) {
	})
}

type recordVerifier[R comparable] struct {
	records map[R]struct{}
	mutex sync.Mutex
	err error
	doneChan chan struct{}
}

func newRecordVerifier[R comparable](ctx context.Context, expectedRecords ...R) *recordVerifier[R] {
	records := make(map[R]struct{})
	for _, r := range expectedRecords {
		records[r] = struct{}{}
	}
	v := &recordVerifier[R]{
		records: records,
		doneChan: make(chan struct{}),
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				v.err = fmt.Errorf("did not receive all records before context expired: %w", ctx.Err())
				close(v.doneChan)
				return
			default:
				if len(v.records) == 0 {
					close(v.doneChan)
					return
				}
			}
		}
	}()
	return v
}

// verify that the received record was expected
// NOTE if a record is received twice, this will return an error.
// This would not be acceptable in a production scenario but for testing
// we can expect to not have duplicates.
func (v *recordVerifier[R]) receive(record R) error {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	if len(v.records) == 0 {
		return fmt.Errorf("received unexpected record %v", record)
	}
	if _, ok := v.records[record]; !ok {
		return fmt.Errorf("received unexpected record %v", record)
	}
	delete(v.records, record)
	return nil
}

func (v *recordVerifier[R]) IsDone() bool {
	select {
	case <-v.doneChan:
		return true
	default:
		return false
	}
}

func (v *recordVerifier[R]) Done() <-chan struct{} {
	return v.doneChan
}

func (v *recordVerifier[R]) Err() error {
	return v.err
}

// Generates group name in the form of "test_[a-z]7" e.g. test_hqbrluz
func randomGroup() string {
	chars := "abcdefghijklmnopqrstuvwxyz"
	length := 7
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return fmt.Sprintf("test_%s", string(b))
}

