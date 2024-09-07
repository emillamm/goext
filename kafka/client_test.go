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
		ctx, _ := context.WithTimeout(context.Background(), 10 * time.Second) // client timeout
		client, err := NewKafkaClient(ctx, env)
		if err != nil { t.Fatal(err) }

		// Create record handler
		handler := NewRecordHandler()
		handler.Handle(func(r *ConsumeRecord) error {
			if v := string(r.Underlying.Value); v != "r1" { return fmt.Errorf("unexpected value %v", v) }
			r.Ack()
			return nil
		})
		handler.Handle(func(r *ConsumeRecord) error {
			if v := string(r.Underlying.Value); v != "r2" { return fmt.Errorf("unexpected value %v", v) }
			r.Ack()
			return nil
		})
		handler.Handle(func(r *ConsumeRecord) error {
			if v := string(r.Underlying.Value); v != "r3" { return fmt.Errorf("unexpected value %v", v) }
			r.Ack()
			return nil
		})
		ctx, _ = context.WithTimeout(ctx, 7 * time.Second) // record handler timeout
		handler.Start(ctx)

		// Configure client
		client.SetGroup(group)

		// register consumer without retry and dlq
		err = client.RegisterConsumer(topic, 0, false, func(record *ConsumeRecord) {
			if err := handler.Receive(record); err != nil {
				t.Error(err)
			}
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

		// publish records
		recordWithValue := func(v string) *kgo.Record { return &kgo.Record{ Value: []byte(v), Topic: topic } }
		if err = client.PublishRecord(ctx, topic, recordWithValue("r1")); err != nil { t.Fatal(err) }
		if err = client.PublishRecord(ctx, topic, recordWithValue("r2")); err != nil { t.Fatal(err) }
		if err = client.PublishRecord(ctx, topic, recordWithValue("r3")); err != nil { t.Fatal(err) }

		// Check and verify consumed records
		select {
		case <-handler.Done():
			if err := handler.Err(); err != nil {
				t.Fatal(err)
			}
		}

		// Close client
		client.CloseGracefylly(ctx)
		client.WaitForDone()
		time.Sleep(10 * time.Millisecond) // allow ErrClientClosed to arrive
		if !client.IsClosed() { t.Fatal("client is not closed") }
	})

	t.Run("publish to topic with retry and dlq", func(t *testing.T) {


		// Initialize values
		topic1WithRetry := "test-topic-1-with-retry"
		topic2WithDlq := "test-topic-2-with-dlq"
		topic3WithRetryAndDlq := "test-topic-3-with-retry-and-dlq"
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
		ctx, _ := context.WithTimeout(context.Background(), 10 * time.Second) // client timeout
		client, err := NewKafkaClient(ctx, env)
		if err != nil { t.Fatal(err) }


		// Create record handlers each topic
		ctx, _ = context.WithTimeout(ctx, 7 * time.Second) // record handler timeout
		handleRecordAck := func(handler *RecordHandler, expectedValue string) {
			handler.Handle(func(r *ConsumeRecord) error {
				if v := string(r.Underlying.Value); v != expectedValue { return fmt.Errorf("unexpected value %v", v) }
				r.Ack()
				return nil
			})
		}
		handleRecordFail := func(handler *RecordHandler, expectedValue string) {
			handler.Handle(func(r *ConsumeRecord) error {
				if v := string(r.Underlying.Value); v != expectedValue { return fmt.Errorf("unexpected value %v", v) }
				r.Fail(ErrRecordFailed)
				return nil
			})
		}

		handler1 := NewRecordHandler() // pass through retry topic two times, then ack
		handleRecordFail(handler1, "r1")
		handleRecordFail(handler1, "r1")
		handleRecordAck(handler1, "r1")
		handler1.Start(ctx)

		handler2 := NewRecordHandler() // pass through dlq topic, then ack
		handleRecordFail(handler2, "r2")
		handleRecordAck(handler2, "r2")
		handler2.Start(ctx)

		handler3 := NewRecordHandler() // pass through retry topic two times, then through dlq topic, then ack
		handleRecordFail(handler3, "r3")
		handleRecordFail(handler3, "r3")
		handleRecordFail(handler3, "r3")
		handleRecordAck(handler3, "r3")
		handler3.Start(ctx)


		// Configure client
		client.SetGroup(group)
		client.SetRetryTopic("test-retry-topic")
		client.SetDlqTopic("test-dlq-topic")


		// register consumers for topics
		registerConsumer := func(handler *RecordHandler, topic string, retries int, useDlq bool) {
			err = client.RegisterConsumer(topic, retries, useDlq, func(record *ConsumeRecord) {
				if err := handler.Receive(record); err != nil {
					t.Error(err)
				}
			})
			if err != nil { t.Fatal(err) }
		}
		registerConsumer(handler1, topic1WithRetry, 2, false)
		registerConsumer(handler2, topic2WithDlq, 0, true)
		registerConsumer(handler3, topic3WithRetryAndDlq, 2, true)


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
		if !client.IsStarted() { t.Fatal("client is not started") }


		// publish records for the three topics
		publishRecordWithValue := func(v string, topic string) {
			r :=  &kgo.Record{ Value: []byte(v), Topic: topic }
			if err = client.PublishRecord(ctx, topic, r); err != nil {
				t.Fatal(err)
			}
		}
		publishRecordWithValue("r1", topic1WithRetry)
		publishRecordWithValue("r2", topic2WithDlq)
		publishRecordWithValue("r3", topic3WithRetryAndDlq)


		// Enable dlq consumption
		if err = client.EnableDlqConsumption(); err != nil {
			t.Fatal(err)
		}


		// Check and verify consumed records
		checkHandler := func(handler *RecordHandler) {
			select {
			case <-handler.Done():
				if err := handler.Err(); err != nil {
					t.Error(err)
				}
			}
		}
		checkHandler(handler1)
		checkHandler(handler2)
		checkHandler(handler3)


		// Close client
		client.CloseGracefylly(ctx)
		client.WaitForDone()
		time.Sleep(10 * time.Millisecond) // allow ErrClientClosed to arrive
		if !client.IsClosed() { t.Fatal("client is not closed") }
	})

	t.Run("return errors", func(t *testing.T) {
	})
}

var ErrRecordFailed = fmt.Errorf("simulated record failure")
type RecordHandler struct {
	handlers []func(*ConsumeRecord)error
	//records map[R]struct{}
	mutex sync.Mutex
	err error
	doneChan chan struct{}
}

func NewRecordHandler() *RecordHandler {
	v := &RecordHandler{
		doneChan: make(chan struct{}),
	}
	return v
}

func (v *RecordHandler) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				v.err = fmt.Errorf("did not receive all records before context expired: %w", ctx.Err())
				close(v.doneChan)
				return
			default:
				if v.Remaining() == 0 {
					close(v.doneChan)
					return
				}
			}
		}
	}()
}

func (v *RecordHandler) Handle(verify func(*ConsumeRecord)error) {
	v.handlers = append(v.handlers, verify)
}

// verify that the received record was expected
// NOTE if a record is received twice, this will return an error.
// This would not be acceptable in a production scenario but for testing
// we can expect to not have duplicates.
func (v *RecordHandler) Receive(record *ConsumeRecord) error {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	if len(v.handlers) == 0 {
		return fmt.Errorf("received unexpected record %v", record)
	}
	// get next handler
	handler := v.handlers[0]
	// remove from queue
	v.handlers = v.handlers[1:len(v.handlers)]
	return handler(record)
}

func (v *RecordHandler) IsDone() bool {
	select {
	case <-v.doneChan:
		return true
	default:
		return false
	}
}

func (v *RecordHandler) Done() <-chan struct{} {
	return v.doneChan
}

func (v *RecordHandler) Err() error {
	return v.err
}

func (v *RecordHandler) Remaining() int {
	v.mutex.Lock()
	r := len(v.handlers)
	v.mutex.Unlock()
	return r
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

