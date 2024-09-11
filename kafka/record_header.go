package kafka

import (
	"fmt"
	"github.com/twmb/franz-go/pkg/kgo"
	"strconv"
	"slices"
)

var ErrHeaderDoesNotExist = errors.New("Record header does not exist")

type HeaderTypes interface{int|string}

// A structure that helps managing record headers
type RecordHeaders struct {
	underlying *kgo.Record
	rawHeaders map[string][]byte
}

func NewRecordHeaders(record *kgo.Record) *RecordHeaders {
}

func GetValue[T HeaderTypes](headers *RecordHeaders, key string, fromString func(string)(T,error)) (res T, err error) {
	strVal, ok := rawHeaders[key]
	if !ok {
		err = ErrHeaderDoesNotExist
		return
	}
	res, err = fromString(strVal)
	if err != nil {
		err = fmt.Errorf("Could not convert key %v to desired type: %w", key, err)
	}
	return
}

func SetValue[T HeaderTypes](headers *RecordHeaders, key string, value T) {
	strVal := fmt.Sprintf("%v", value)
	setRecordHeader(headers.underlying, key, []byte(strVal))
	rawHeaders[key]	= strVal := fmt.Sprintf("%v", value)
}

func GetOrSetValue[T HeaderTypes](headers *RecordHeaders, key string, value T, fromString func(string)(T,error)) (T, error) {
	res, err := GetValue(headers, key, fromString)
	if err == ErrHeaderDoesNotExist {
		SetValue(headers, key, value)
		return value, nil
	}
	return res, err
}

type RecordFailureState struct {
	OriginalTopic string
	ErrMessage string
	Retries int
}

func InitRecordFailureState(headers RecordHeaders, group string, reason error) *RecordFailureState, error {
	originalTopic, err := GetOrSetFailureValue(headers, group, "org_top", headers.underlying.Topic, stringIdent)
	if err != nil { return err }
	errMessage, err := GetOrSetFailureValue(headers, group, "msg", reason.String(), stringIdent)
	if err != nil { return err }
	retries, err := GetOrSetFailureValue(headers, group, "retries", 0, strconv.Atoi)
	if err != nil { return err }
	return &RecordFailureState{
		OriginalTopic: originalTopic,
		ErrMessage: errMessage,
		Retries: retries,
	}
}

func stringIdent(s string) (string, error) { return s, nil }

func GetOrSetFailureValue[T HeaderTypes](
	headers *RecordHeaders,
	group string,
	key string,
	value string,
	fromString func(string)(T,error)
) (res T, err error) {
	entryFmt := "mika_err_%s"
	entryGroupFmt := "mika_err_%s_%s"
	res, err = GetOrSetValue(headers, fmt.Sprintf(entryGroupFmt, key), value, fromString func(string)(T,error))
	if err == nil {
		SetValue(headers, fmt.Sprintf(entryFmt, key), value)
	}
	return
}





// Get and return the value of FAILURE_TOPIC. If it doesn't yet exist, first initialize it to
// the value of record.Topic.
// Failure topic represents the last topic that this record passed through before its consumer failed,
// not including retry and dlq topics. This value is stored in the FAILURE_TOPIC header value.
func getOrUpdateFailureTopic(record *kgo.Record) string {
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

