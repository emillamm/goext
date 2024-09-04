package kafkahelper

import (
	"errors"
	"sync"
	"context"
	"fmt"
)

var ErrConsumerTopicAlreadyExists = errors.New("Cannot add consumer for a topic that is already registered")
var ErrConsumerTopicDoesntExist = errors.New("Cannot fetch consumer for topic")
var ErrConsumerTopicsOutOfSync = errors.New("Consumer registry topics are out of sync with the broker")

// consumer registry is a goroutine safe implementation that keeps track of registered consumers
type ConsumerRegistry struct {
	mutex sync.Mutex
	consumers map[string]*RegisteredConsumer
}

type RegisteredConsumer struct {
	Enabled bool
	Retries int
	UseDlq bool
	Process func(*ConsumeRecord)
}


func NewConsumerRegistry() *ConsumerRegistry {
	return &ConsumerRegistry{
		consumers: make(map[string]*RegisteredConsumer),
	}
}

func (c *ConsumerRegistry) ConsumerTopics() []string {
	topics := make([]string, len(c.consumers))
	i := 0
	for t := range c.consumers {
		topics[i] = t
	    	i++
    	}
	return topics
}

func (c *ConsumerRegistry) EnabledConsumerTopics() []string {
	var topics []string
	for t, v := range c.consumers {
		if v.Enabled {
			topics = append(topics, t)
		}
    	}
	return topics
}

// Adds topic and consumer to registry. If a topic already exists in the registry, returns ErrConsumerTopicAlreadyExists.
// Otherwise returns nil.
// Registered consumers are enabled by default.
func (c *ConsumerRegistry) AddConsumer(
	topic string,
	retries int,
	useDlq bool,
	process func(*ConsumeRecord),
) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if _, ok := c.consumers[topic]; ok {
		return ErrConsumerTopicAlreadyExists
	}
	c.consumers[topic] = &RegisteredConsumer{
		Enabled: true,
		Retries: retries,
		UseDlq: useDlq,
		Process: process,
	}
	return nil
}

// TODO
// 1. Return a channel of error as a callback to know whether the action completed successfully
// 2. Add a new method to "sync" the map based on a list of enabled topics. This ensures consistency.
// Set enable/disable on consumer. Return ErrConsumerTopicDoesntExist if it doesn't exist.
// The enabled flag and action is eventually consistent as the action completes async.
// If the action eventually returns an error, the value of enabled will not be changed.
// If the provided context expires before the action completed, the goroutine is aborted. This
// might lead to inconsistencies given the unknown outcome of action. I.e. since it is a network call,
// it might still complete successfully from the perspective of the broker. 
// onSuccess is called only if the action doesn't return an error and the context hasn't expired.
//func (c *ConsumerRegistry) SetEnabled(
//	ctx context.Context,
//	topic string,
//	enabled bool,
//	action func()error,
//) <-chan error {
//	if err := ctx.Err(); err != nil {
//		return err
//	}
//	c.mutex.Lock()
//	defer c.mutex.Unlock()
//	consumer, ok := c.consumers[topic]
//	if !ok {
//		return ErrConsumerTopicDoesntExist
//	}
//	if consumer.enabled == enabled {
//		return nil // no-op
//	}
//	// perform action async
//	actionErrChan := make(chan error)
//	resultErrChan := make(chan error)
//	go func() {
//		actionErrChan <-action()
//	}()
//	go func() {
//		select {
//		case err := <-actionErrChan:
//			if err == nil {
//				consumer.enabled = enabled
//			}
//		case <-ctx.Done():
//			break
//		}
//	}()
//	return nil
//}

// mutex will remain locked for as long as it takes for the sync operation to finish or for the context to expire
func (c *ConsumerRegistry) SetEnabled(
	ctx context.Context,
	topic string,
	enabled bool,
	syncOp func()error,
) <-chan error {
	errChan := make(chan error)
	wrapErr := func(err error) error { return fmt.Errorf("could not set topic %s enabled=%b: %w", topic, enabled, err) }
	if err := ctx.Err(); err != nil {
		errChan <- wrapErr(err)
		return errChan
	}
	c.mutex.Lock()
	consumer, ok := c.consumers[topic]
	if !ok {
		errChan <-wrapErr(ErrConsumerTopicDoesntExist)
		c.mutex.Unlock()
		return errChan
	}
	if consumer.Enabled == enabled {
		errChan <-nil // no-op
		c.mutex.Unlock()
		return errChan
	}
	// set enabled for topic in registry (locally)
	consumer.Enabled = enabled
	// perform sync operation (async)
	syncOpChan := make(chan error)
	go func() {
		syncOpChan <-syncOp()
	}()
	go func() {
		// Wait for sync operation to finish, fail or context expire.
		select {
		case err := <-syncOpChan:
			if err != nil {
				err = wrapErr(ErrConsumerTopicsOutOfSync)
			}
			errChan <-err
		case <-ctx.Done():
			errChan <-wrapErr(ctx.Err())
		}
		c.mutex.Unlock()
	}()
	return errChan
}

// Get consumer from topic or return ErrConsumerTopicDoesntExist if it doesn't exist.
func (c *ConsumerRegistry) GetConsumer(topic string) (*RegisteredConsumer, error) {
	consumer, ok := c.consumers[topic]
	if !ok {
		return nil, ErrConsumerTopicDoesntExist
	}
	return consumer, nil
}

