package kafka

import (
	"testing"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestRecordHeaderFunctions(t *testing.T) {

	t.Run("set and get record header", func(t *testing.T) {
		record := &kgo.Record{}
		if h := getRecordHeader(record, "foo"); h != nil {
			t.Errorf("expected nil header")
		}
		setRecordHeader(record, "foo", []byte("bar"))
		if h := getRecordHeader(record, "foo"); h == nil || string(h) != "bar" {
			t.Errorf("got %v, want bar", h)
		}

	})

	t.Run("increment and get retry attempts", func(t *testing.T) {
		record := &kgo.Record{}
		if r, err := getRetryAttempts(record); err !=nil || r != 0 {
			t.Errorf("got %v, %v, want nil, 0", r, err)
		}
		if h := getRecordHeader(record, "RETRY_ATTEMPTS"); h != nil {
			t.Errorf("got %v, want []", h)
		}
		if r, err := incrementRetryAttempts(record); err !=nil || r != 1 {
			t.Errorf("got %v, %v, want nil, 0", r, err)
		}
		if h := getRecordHeader(record, "RETRY_ATTEMPTS"); h == nil || string(h) != "1" {
			t.Errorf("got %v, want 1", h)
		}
		if r, err := getRetryAttempts(record); err !=nil || r != 1 {
			t.Errorf("got %v, %v, want nil, 1", r, err)
		}

	})

	t.Run("update and get failure topic", func(t *testing.T) {
		record := &kgo.Record{}
		if topic := getFailureTopic(record); topic != "" {
			t.Errorf("got %v, want \"\"", topic)
		}
		record.Topic = "foo"
		if topic := getOrUpdateFailureTopic(record); topic != "foo" {
			t.Errorf("got %v, want foo", topic)
		}
		if topic := getFailureTopic(record); topic != "foo" {
			t.Errorf("got %v, want foo", topic)
		}
		// should still be foo after updating topic to bar
		record.Topic = "bar"
		if topic := getOrUpdateFailureTopic(record); topic != "foo" {
			t.Errorf("got %v, want foo", topic)
		}
		if topic := getFailureTopic(record); topic != "foo" {
			t.Errorf("got %v, want foo", topic)
		}
	})
}

