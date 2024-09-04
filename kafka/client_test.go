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

	now := time.Now()
	ctx := context.Background()

	t.Run("", func(t *testing.T) {
	})

	t.Run("publish and consume records", func(t *testing.T) {
		ctx, _ = context.WithTimeout(ctx, 10 * time.Second)
		client, err := NewKafkaClient(ctx, os.Getenv)
		if err != nil { t.Fatal(err) }
		println(fmt.Sprintf("timecheck 1 %v", time.Since(now).Milliseconds()))

		//doneChan := make(chan struct{})

		ctx, _ = context.WithTimeout(ctx, 7 * time.Second)
		verifier := newRecordVerifier[string](ctx, "a")

		err = client.RegisterConsumer("test-topic-11", 0, false, func(record *ConsumeRecord) {
			//println("record received")
			println(fmt.Sprintf("record received %v", time.Since(now).Milliseconds()))
			verifier.receive(string(record.Underlying.Value))
			record.Ack()
		})
		if err != nil {
			t.Fatal(err)
		}

		println(fmt.Sprintf("timecheck 2 %v", time.Since(now).Milliseconds()))

		group := randomGroup()
		println(group)
		client.SetGroup(group)

		// start
		errs := client.Start()
		println(fmt.Sprintf("timecheck 3 %v", time.Since(now).Milliseconds()))
		go func() {
			for err := range errs {
				t.Error(err)
				if errors.Is(err, ErrClientClosed) {
					//close(doneChan)
					return
				}
			}
		}()

		record := &kgo.Record{
			Value: []byte("a"),
			Topic: "test-topic-11",
		}

		if err = client.PublishRecord(ctx, "test-topic-11", record); err != nil {
			t.Fatal(err)
		}

		select {
		case <-verifier.Done():
			if err := verifier.Err(); err != nil {
				t.Fatal(err)
			}
		}

		// TODO add solution to this
		// Add graceful timeout to client.Close()
		client.CloseGracefylly(ctx)
		client.WaitForDone()
		//<-doneChan
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
		println("end")
	}()
	return v
}

func (v *recordVerifier[R]) receive(record R) {
	println(fmt.Sprintf("received %v", record))
	v.mutex.Lock()
	delete(v.records, record)
	v.mutex.Unlock()
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

