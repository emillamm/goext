package kafka

import (
	"testing"
	"github.com/twmb/franz-go/pkg/kgo"
	"strconv"
	"errors"
)

func TestHeaders(t *testing.T) {

	t.Run("create RecordHeaders from initially empty headers", func(t *testing.T) {
		h := NewHeaders(&kgo.Record{})
		if len(h.headerMap) > 0 {
			t.Errorf("headermap should be empty")
		}
	})

	t.Run("create RecordHeaders from non-empty headers", func(t *testing.T) {
		h := NewHeaders(&kgo.Record{
			Headers: []kgo.RecordHeader{
				kgo.RecordHeader{
					Key: "k1",
					Value: []byte("v1"),
				},
				kgo.RecordHeader{
					Key: "k2",
					Value: []byte("v2"),
				},
			},
		})
		if len(h.headerMap) != 2 {
			t.Errorf("headermap contain one element")
		}
		header1 := h.headerMap["k1"]
		if header1.Index != 0 || string(header1.Bytes) != "v1" {
			t.Errorf("invalid value for header1: %v", *header1)
		}
		header2 := h.headerMap["k2"]
		if header2.Index != 1 || string(header2.Bytes) != "v2" {
			t.Errorf("invalid value for header2: %v", *header2)
		}
	})

	t.Run("get and set headers", func(t *testing.T) {
		// start with initially empty record
		h := NewHeaders(&kgo.Record{})

		report := func(value any, err error) {
			t.Helper()
			t.Errorf("invalid value returned: %v error: %v", value, err)
		}
		// Set and get string values
		SetHeader(h, "k1", "v1", StringSerializer)
		if v, err := GetHeader(h, "k1", StringDeserializer); err != nil || v != "v1" {
			report(v, err)
		}

		// Set and get int value
		SetHeader(h, "k2", 2, IntSerializer)
		if v, err := GetHeader(h, "k2", IntDeserializer); err != nil || v != 2 {
			report(v, err)
		}

		// GetOrSet new value
		if v, err := GetOrSetHeader(h, "k3", "v3", StringDeserializer, StringSerializer); err != nil || v != "v3" {
			report(v, err)
		}

		// GetOrSet for existing key
		if v, err := GetOrSetHeader(h, "k3", "foo", StringDeserializer, StringSerializer); err != nil || v != "v3" {
			report(v, err)
		}

		// Overwrite existing matching type
		SetHeader(h, "k1", "foo", StringSerializer)
		if v, err := GetHeader(h, "k1", StringDeserializer); err != nil || v != "foo" {
			report(v, err)
		}

		// Overwrite existing different type
		SetHeader(h, "k1", 999, IntSerializer)
		if v, err := GetHeader(h, "k1", IntDeserializer); err != nil || v != 999 {
			report(v, err)
		}

		// Get value different type
		if v, err := GetHeader(h, "k3", IntDeserializer); v != 0 || !errors.Is(err, strconv.ErrSyntax) {
			report(v, err)
		}

		// Get non-existing key
		if v, err := GetHeader(h, "k4", StringDeserializer); v != "" || !errors.Is(err, ErrHeaderDoesNotExist) {
			report(v, err)
		}

	})

	//t.Run("set and get record header", func(t *testing.T) {
	//	record := &kgo.Record{}
	//	if h := getRecordHeader(record, "foo"); h != nil {
	//		t.Errorf("expected nil header")
	//	}
	//	setRecordHeader(record, "foo", []byte("bar"))
	//	if h := getRecordHeader(record, "foo"); h == nil || string(h) != "bar" {
	//		t.Errorf("got %v, want bar", h)
	//	}

	//})

	//t.Run("increment and get retry attempts", func(t *testing.T) {
	//	record := &kgo.Record{}
	//	if r, err := getRetryAttempts(record); err !=nil || r != 0 {
	//		t.Errorf("got %v, %v, want nil, 0", r, err)
	//	}
	//	if h := getRecordHeader(record, "RETRY_ATTEMPTS"); h != nil {
	//		t.Errorf("got %v, want []", h)
	//	}
	//	if r, err := incrementRetryAttempts(record); err !=nil || r != 1 {
	//		t.Errorf("got %v, %v, want nil, 0", r, err)
	//	}
	//	if h := getRecordHeader(record, "RETRY_ATTEMPTS"); h == nil || string(h) != "1" {
	//		t.Errorf("got %v, want 1", h)
	//	}
	//	if r, err := getRetryAttempts(record); err !=nil || r != 1 {
	//		t.Errorf("got %v, %v, want nil, 1", r, err)
	//	}

	//})

	//t.Run("update and get failure topic", func(t *testing.T) {
	//	record := &kgo.Record{}
	//	if topic := getFailureTopic(record); topic != "" {
	//		t.Errorf("got %v, want \"\"", topic)
	//	}
	//	record.Topic = "foo"
	//	if topic := getOrUpdateFailureTopic(record); topic != "foo" {
	//		t.Errorf("got %v, want foo", topic)
	//	}
	//	if topic := getFailureTopic(record); topic != "foo" {
	//		t.Errorf("got %v, want foo", topic)
	//	}
	//	// should still be foo after updating topic to bar
	//	record.Topic = "bar"
	//	if topic := getOrUpdateFailureTopic(record); topic != "foo" {
	//		t.Errorf("got %v, want foo", topic)
	//	}
	//	if topic := getFailureTopic(record); topic != "foo" {
	//		t.Errorf("got %v, want foo", topic)
	//	}
	//})
}

