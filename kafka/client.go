package kafka

import (
	"context"
	"fmt"
	"github.com/emillamm/envx"
	"github.com/emillamm/goext/kafkahelper"
	"github.com/twmb/franz-go/pkg/kgo"
	"sync"
	"errors"
	"time"
	mapset "github.com/deckarep/golang-set/v2"
)

var ErrClientClosed = errors.New("KafkaClient is closed")

// Exported types from kafkahelper module
var ErrConsumerTopicAlreadyExists = kafkahelper.ErrConsumerTopicAlreadyExists
var ErrConsumerTopicDoesntExist = kafkahelper.ErrConsumerTopicDoesntExist
type ConsumeRecord = kafkahelper.ConsumeRecord


type KafkaClient struct {
	consumerRegistry *kafkahelper.ConsumerRegistry
	consumerStatus *kafkahelper.ConsumerStatus
	// error channel - this is never closed after the client is created
	errs chan error
	underlying *kgo.Client
	group string
	retryTopic string
	dlqTopic string
	startedChan chan struct{}
	doneChan chan struct{}
	doneWaitChan chan struct{}
	startOffset kgo.Offset
}

// Create a new kafka client that will shutdown (not gracefully) if the context expires.
// In order to shutdown gracefully, i.e. finish processing and committing fetched records,
// call client.CloseGracefully(ctx) with a context that dictates the graceful shutdown period.
func NewKafkaClient(ctx context.Context, env envx.EnvX) (client *KafkaClient, err error) {

	// Initialize offset based on value of "KAFKA_CONSUMER_START_FROM"
	startOffset, err := getConsumerStartOffset(env)
	if err != nil {
		return nil, err
	}

	client = &KafkaClient{
		consumerRegistry: kafkahelper.NewConsumerRegistry(),
		consumerStatus: kafkahelper.NewConsumerStatus(),
		errs: make(chan error),
		startedChan: make(chan struct{}),
		doneChan: make(chan struct{}),
		doneWaitChan: make(chan struct{}),
		startOffset: startOffset,
	}

	// async shutdown
	go func() {
		// wait for context to be done or for client done channel to be closed
		select {
		case <-ctx.Done():
			client.Close()
		case <-client.doneChan:
			break
		}
		// close underlying client
		client.underlying.Close()
		// signal that wait is over and client is now closed
		client.errs <- ErrClientClosed
		close(client.doneWaitChan)
	}()

	return
}

// Register a topic and consumer. If a topic already exists in the registry, returns ErrConsumerTopicAlreadyExists,
// otherwise returns nil. Consumers must be registered before the client is started, otherwise it panics.
// A consumer is enabled by default when registered.
func (k *KafkaClient) RegisterConsumer(
	topic string,
	retries int,
	useDlq bool,
	process func(*ConsumeRecord),
) error {
	if k.IsStarted() {
		panic("cannot register consumer after client has been started")
	}
	return k.consumerRegistry.AddConsumer(
		topic,
		retries,
		useDlq,
		process,
	)
}

// Set client consumer group. Must be called before client is started otherwise it will panic.
func (k *KafkaClient) SetGroup(group string) {
	if k.IsStarted() {
		panic("cannot set consumer group after client has been started")
	}
	k.group = group
}

func (k *KafkaClient) SetRetryTopic(topic string) {
	if k.IsStarted() {
		panic("cannot set consumer group after client has been started")
	}
	if k.retryTopic != "" {
		panic("cannot set retry topic after it was already set")
	}
	k.retryTopic = topic
}

func (k *KafkaClient) SetDlqTopic(topic string) {
	if k.IsStarted() {
		panic("cannot set consumer group after client has been started")
	}
	if k.dlqTopic != "" {
		panic("cannot set dlq topic after it was already set")
	}
	k.dlqTopic = topic
}

// Setup client connection to brokers. If any consumers are registered, start consuming immediately.
// Calling this if the client is already started, will not have an effect.
// Calling Start if the client is closed, will not have an effect.
// The returned error channel is unbuffered and needs to have a listener for the full lifecycle of the client.
// Otherwise, the client will block when trying to push errors onto the channel.
func (k *KafkaClient) Start() (errs <-chan error) {

	errs = k.errs

	if (k.IsStarted()) {
		return
	}

	defer close(k.startedChan)

	if k.IsClosed() {
		return
	}

	// load enabled consume topcis
	consumeTopics := k.consumerRegistry.EnabledConsumerTopics()

	// add retry topic for consumption
	if k.retryTopic != "" {
		consumeTopics = append(consumeTopics, k.retryTopic)
	}

	// create underlying *kgo.Client
	underlying, err := LoadKgoClient(consumeTopics, k.group, k.startOffset)
	if err != nil {
		k.errs <- fmt.Errorf("failed to initialize *kgo.Client with error: %w", err)
		k.Close()
		return
	}
	k.underlying = underlying


	// Start consuming regardless if there are any registered enabled consumer topics.
	// If not, it will not poll.
	k.startConsuming()

	return
}

// Enable consumption of topic if it was previously disabled. This means resuming
// the consumer if it was disabled after the client was started or adding the consumer
// (for the first time) if it was disabled before the client was started.
// If the consumer is already enabled, this is a no-op.
// If topic was never registered, this will return ErrConsumerTopicDoesntExist.
// If requires remote changes, a call to SyncConsumerTopics() is made.
// If the sync operation times out (context expires), the registry might be out of sync
// with the broker. An error is returned in this case and it is up to the caller to
// handle the error by retrying the sync operation for example.
func (k *KafkaClient) EnableConsumerTopic(ctx context.Context, topic string) error {
	return <-k.consumerRegistry.SetEnabled(ctx, topic, true, k.SyncConsumerTopics)
}

// Disable consumption of topic if it was previously enabled. This means pausing
// the consumer if it was disabled after the client was started or removing the consumer
// if it was disabled before the client was started.
// If the consumer is already disabled, this is a no-op.
// If topic was never registered, this will return ErrConsumerTopicDoesntExist.
// If requires remote changes, a call to SyncConsumerTopics() is made.
// If the sync operation times out (context expires), the registry might be out of sync
// with the broker. An error is returned in this case and it is up to the caller to
// handle the error by retrying the sync operation for example.
func (k *KafkaClient) DisableConsumerTopic(ctx context.Context, topic string) error {
	return <-k.consumerRegistry.SetEnabled(ctx, topic, false, k.SyncConsumerTopics)
}

// Pause/resume consumption of topics in Kafka according to the status of the consumers in the registry.
func (k *KafkaClient) SyncConsumerTopics() error {
	currentPausedTopics := k.underlying.PauseFetchTopics() // no args returns all paused topics
	allTopics := mapset.NewSet[string](k.consumerRegistry.ConsumerTopics()...)
	enabledTopics := mapset.NewSet[string](k.consumerRegistry.EnabledConsumerTopics()...)
	pausedTopics := mapset.NewSet[string](currentPausedTopics...)
	topicsToPause := allTopics.Difference(enabledTopics).Difference(pausedTopics)
	topicsToResume := enabledTopics.Intersect(pausedTopics)
	k.underlying.PauseFetchTopics(topicsToPause.ToSlice()...)
	k.underlying.ResumeFetchTopics(topicsToResume.ToSlice()...)
	return nil
}

func (k *KafkaClient) EnableDlqConsumption() error {
	if k.dlqTopic == "" {
		return fmt.Errorf("Client is not configured with a dlq")
	}
	k.underlying.AddConsumeTopics(k.dlqTopic)
	k.underlying.ResumeFetchTopics(k.dlqTopic)
	return nil
}

func (k *KafkaClient) DisableDlqConsumption() error {
	if k.dlqTopic == "" {
		return fmt.Errorf("Client is not configured with a dlq")
	}
	k.underlying.PauseFetchTopics(k.dlqTopic)
	return nil
}

// Produce a record to the give topic.
// If the provided context expures, the method will fail and return an error.
// If the client is closed, an error will also be returned.
// If the client is currently terminating gracefully, publishing will be allowed
// for as long as the underlying client is alive.
func (k *KafkaClient) PublishRecord(
	ctx context.Context,
	topic string,
	record *kgo.Record,
) (err error) {
	var wg sync.WaitGroup
	wg.Add(1)
	k.underlying.Produce(ctx, record, func(_ *kgo.Record, produceErr error) {
		defer wg.Done()
		if produceErr != nil {
			err = fmt.Errorf("failed to produce record: %w", produceErr)
		}
	})
	wg.Wait()
	return
}

// No-op if client is already closed. Otherwise stop consumption and close underlying client.
// When the client is fully closed, ErrClientClosed will be returned via the error channel.
func (k *KafkaClient) Close() {
	select {
	case <-k.doneChan:
		return
	default:
		k.consumerStatus.Terminate()
		close(k.doneChan)
	}
}

// Close client by stopping consumption gracefully if consoumer is actively consuming and
// not terminating. Otherwise, this will close client normally.
func (k *KafkaClient) CloseGracefylly(ctx context.Context) {
	go func() {
		k.consumerStatus.TerminateGracefully(ctx)
		select {
		case <-k.doneChan:
			k.consumerStatus.Terminate()
			return
		case <-k.consumerStatus.DoneSig():
			k.Close()
		}
	}()
}

// Returns true if client.Start() has been called, false otherwise.
func (k *KafkaClient) IsStarted() bool {
	select {
	case <-k.startedChan:
		return true
	default:
		return false
	}
}

// Returns true if the client is closed, false otherwise.
func (k *KafkaClient) IsClosed() bool {
	select {
	case <-k.doneChan:
		return true
	default:
		return false
	}
}

// Wait for client to be fully closed
func (k *KafkaClient) WaitForDone() {
	<-k.doneWaitChan
}

// Read value of KAFKA_CONSUMER_START_FROM as a timestamp.
// If the value is present, convert it to a timestamp that will be used
// as offset for new consumer groups. This will be ignored by existing consumer groups.
// If the value is not present, default to the .AtCommitted() offset, or auto.offset.reset "none"
// in Kafka. Given this configuration, new consumer groups will fail if the value is not present.
func getConsumerStartOffset(env envx.EnvX) (kgo.Offset, error) {
	var err error
	startFrom := env.AsTime(time.RFC3339).Getenv("KAFKA_CONSUMER_START_FROM", envx.Intercept[time.Time](&err))
	if err != nil {
		if errors.Is(err, envx.ErrEmptyValue) {
			return kgo.NewOffset().AtCommitted(), nil
		}
		return kgo.Offset{}, fmt.Errorf("invalid format of KAFKA_CONSUMER_START_FROM: %w", err)
	}
	startOffset := kgo.NewOffset().AfterMilli(startFrom.UnixMilli())
	return startOffset, nil
}

// Reset consumer status and start the poll, process, commit loop.
// Calling this if the client is already consuming, will not have an effect.
// Calling this if the client is closed, will not have an effect.
func (k *KafkaClient) startConsuming() {

	if k.IsClosed() || k.consumerStatus.IsConsuming() {
		return
	}

	// initialize consumer done channel and start polling
	k.consumerStatus.Reset()
	k.consumerStatus.Start()
	k.startPollProcessCommitLoop()
}

// Start poll, process commit loop.
// It will run forever until the consumer done channel is closed.
// It will short-circuit and retry if there are no enabled consumer topics.
func (k *KafkaClient) startPollProcessCommitLoop() {
	go func() {
		for {
			if k.IsClosed() || k.consumerStatus.IsDone() {
				return
			}
			if k.consumerStatus.IsTerminating() {
				k.consumerStatus.Terminate()
				return
			}
			if len(k.consumerRegistry.EnabledConsumerTopics()) > 0 {
				k.pollProcessCommit()
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (k *KafkaClient) pollProcessCommit() {

	// Create a context that can be passed to kgo.Client.PollFetches
	// and will be cancelled if the consumer done channel is closed.
	// Put this in a go routine that will end once the polling finishes.
	doneChan := make(chan struct{})
	defer close(doneChan)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-k.consumerStatus.DoneSig():
			cancel()
		case <-doneChan:
			break
		}
	}()

	// This waitgroup ensures that the method blocks until all records have been processed.
	var wg sync.WaitGroup
	fetches := k.underlying.PollFetches(ctx)

	// if there are any errors, publish them all to the errs channel and close client without committing
	if fetches.Err() != nil {
		fetches.EachError(func(topic string, partition int32, err error) {
			k.errs <- fmt.Errorf("fetch error at topic %s, partition %v: %w", topic, partition, err)
		})
		k.Close()
		return
	}

	fetches.EachRecord(func(record *kgo.Record) {
		wg.Add(1)
		// get consumer handler from topic
		consumer, err := k.getConsumerForRecord(record)
		if err != nil {
			panic(fmt.Sprintf("failed to get consumer for topic %s: %v", record.Topic, err))
		}

		// prepare record
		var ackOnce sync.Once
		ack := func() {
			ackOnce.Do(func() {
				wg.Done()
			})
		}
		var failOnce sync.Once
		fail := func(reason error) {
			failOnce.Do(func() {
				if err := k.handleFailedRecord(record, reason); err != nil {
					k.errs <- fmt.Errorf("calling fail() on a record resulted in an error which can lead to blocked consumers: %w", err)
				} else {
					ack()
				}
			})
		}

		consumer.Process(kafkahelper.NewConsumeRecord(record, ack, fail))
	})
	wg.Wait()
	// Try committing offsets. If it fails, publish error and close client
	if err := k.underlying.CommitUncommittedOffsets(ctx); err != nil {
		k.errs <- fmt.Errorf("failed to commit offsets - closing client: %w", err)
		k.Close()
	}
}

func (k *KafkaClient) getConsumerForRecord(record *kgo.Record) (consumer *kafkahelper.RegisteredConsumer, err error) {
	consumer, err = k.consumerRegistry.GetConsumer(record.Topic)
	// if consumer doesn't exist for topic, try getting consumer from FAILURE_TOPIC, if present.
	// This will find the right consumer if the record came from a retry/dlq topic.
	if err != nil && errors.Is(err, ErrConsumerTopicDoesntExist) {
		failureTopic := getFailureTopic(record)
		if failureTopic != "" {
			consumer, err = k.consumerRegistry.GetConsumer(failureTopic)
		}
	}
	return
}

func (k *KafkaClient) handleFailedRecord(record *kgo.Record, failureReason error) error {
	topic := updateFailureTopic(record)
	retries, err := getRetryAttempts(record)
	if err != nil {
		return err
	}
	consumer, err := k.consumerRegistry.GetConsumer(topic)
	if err != nil {
		return fmt.Errorf("could not get consumer for topic %s: %w", topic, err)
	}

	// publish to retry topic if one exists and there are retries left
	if k.retryTopic != "" && consumer.Retries > retries {
		ctx, _ := context.WithTimeout(context.Background(), 5 * time.Second) // TODO make configurable
		retryRecord := *record
		retryRecord.Topic = k.retryTopic
		incrementRetryAttempts(&retryRecord)
		if err := k.PublishRecord(ctx, k.retryTopic, &retryRecord); err != nil {
			return fmt.Errorf("failed to publish record to retry topic: %w", err)
		}
		return nil
	}
	if consumer.UseDlq  && k.dlqTopic != "" {
		ctx, _ := context.WithTimeout(context.Background(), 5 * time.Second) // TODO make configurable
		dlqRecord := *record
		dlqRecord.Topic = k.dlqTopic
		if err := k.PublishRecord(ctx, k.dlqTopic, &dlqRecord); err != nil {
			return fmt.Errorf("failed to publish record to dlq topic: %w", err)
		}
		return nil
	}
	return fmt.Errorf("dlq not enabled for topic %s. Record failed with error: %s", record.Topic, failureReason)
}

