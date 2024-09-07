package kafka

import (
	"fmt"
	"github.com/twmb/franz-go/pkg/kgo"
	"strconv"
	"slices"
)

// Get and return the value of FAILURE_TOPIC. If it doesn't yet exist, first initialize it to
// the value of record.Topic.
// Failure topic represents the last topic that this record passed through before its consumer failed,
// not including retry and dlq topics. This value is stored in the FAILURE_TOPIC header value.
func updateFailureTopic(record *kgo.Record) string {
	h := getRecordHeader(record, "FAILURE_TOPIC")
	if h == nil {
		setRecordHeader(record, "FAILURE_TOPIC", []byte(record.Topic))
		return record.Topic
	}
	return string(h)
}

func getFailureTopic(record *kgo.Record) string {
	if h := getRecordHeader(record, "FAILURE_TOPIC"); h != nil {
		return string(h)
	} else {
		return ""
	}
}

// Get number of attempted retries from RETRY_ATTEMPTS header value.
// Return 0 if it doesn't exist or an error if the value cannot be converted to an int.
func getRetryAttempts(record *kgo.Record) (retries int, err error) {
	retryBytes := getRecordHeader(record, "RETRY_ATTEMPTS")
	if retryBytes == nil {
		return
	}
	retryStr := string(retryBytes)
	retries, err = strconv.Atoi(retryStr)
	if err != nil {
		err = fmt.Errorf("could not convert header value of RETRY_ATTEMPTS %s to an int: %v", retryStr, err)
	}
	return
}

// Increment value of attempted retries by overwriting RETRY_ATTEMPTS header value.
// If value doesn't exist, it will be initialized to 1.
// Return the incremented value.
func incrementRetryAttempts(record *kgo.Record) (retries int, err error) {
	retries, err = getRetryAttempts(record)
	if err != nil {
		return
	}
	retries = retries + 1
	setRecordHeader(record, "RETRY_ATTEMPTS", []byte(strconv.Itoa(retries)))
	return
}

// Get header from key as a byte array. Return empty if key doesn't exist.
func getRecordHeader(record *kgo.Record, key string) []byte {
	i := slices.IndexFunc(record.Headers, func(r kgo.RecordHeader) bool {
		return r.Key == key
	})
	if i > -1 {
		return record.Headers[i].Value
	}
	return nil
}

// Set or overwrite header for key
func setRecordHeader(record *kgo.Record, key string, value []byte) {
	headers := record.Headers
	i := slices.IndexFunc(headers, func(r kgo.RecordHeader) bool {
		return r.Key == key
	})
	header := kgo.RecordHeader{Key: key, Value: value}
	if i > -1 {
		headers[i] = header
	} else {
		headers = append(headers, header)
	}
	record.Headers = headers
}

