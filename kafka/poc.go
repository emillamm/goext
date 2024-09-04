package kafka

//import (
//	"context"
//	"fmt"
//	"google.golang.org/protobuf/proto"
//	"github.com/twmb/franz-go/pkg/kgo"
//	"sync"
//	"time"
//)
//
//// Only one consumer per topic.
//// TODO if we are not already consuming (because only producers when client was created) then call startConsuming.
//func (k *KafkaClient) OpenConsumer(
//	topic string,
//	onFail func(*kgo.Record)error,
//	process func(ConsumeRecord),
//) {
//
//	// Add topic and consumer to map if it doesn't exist
//	k.consumers.Store(topic, &consumerHandler{
//		onFail: onFail,
//		process: process,
//	})
//
//	// TODO check if running
//	// Add consume topics if client is already consuming
//	k.underlying.AddConsumeTopics(topic)
//}
//
//
//
//func (b *KafkaClientBuilder) RegisterProducer(topic string) KafkaProducer {
//}
//
//func (k *KafkaProducer) Produce(context.Context, *kgo.Record) error
//
//
//
//
//
//	
//
//// A producer client is unaware of topics
//func NewProducerClient(
//) *kgo.Client {
//	return nil
//}
//
//type Kafka struct {
//	Client *kgo.Client
//}
//
//func PublishMessage(
//	ctx context.Context,
//	client *kgo.Client,
//	topic string,
//	message proto.Message,
//) (err error) {
//	bytes, err := proto.Marshal(message)
//	if err != nil {
//		return
//	}
//	record := &kgo.Record{Topic: topic, Value: bytes}
//	err = PublishRecord(ctx, client, topic, record)
//	return
//}
//
//func PublishRecord(
//	ctx context.Context,
//	client *kgo.Client,
//	topic string,
//	record *kgo.Record,
//) (err error) {
//	ctx, _ = context.WithTimeout(ctx, 5 * time.Second)
//	println(2)
//	var wg sync.WaitGroup
//	wg.Add(1)
//	client.Produce(ctx, record, func(_ *kgo.Record, produceErr error) {
//		println(3)
//		defer wg.Done()
//		if produceErr != nil {
//			err = fmt.Errorf("kafka produce error: %w", produceErr)
//		}
//	})
//	wg.Wait()
//	return
//}
//
////func (k *Kafka) ConsumeMessages[T any](
////	ctx context.Context,
////	topic string,
////	process func(*kgo.Record, error, func(), func()),
////) (errs chan error) {
////	var t T
////	fn := func(record *kgo.Record, ack func(), fail func()) {
////		message := 
////	}
////	return ConsumeRecords(ctx, topic, 
////}
//
//// TODO
//// check errors
//// handle context expire
//func ConsumeRecords(
//	ctx context.Context,
//	client *kgo.Client,
//	topic string,
//	process func(*kgo.Record, func(), func()),
//) (errs chan error) {
//	errs = make(chan error)
//	fetches := client.PollFetches(ctx)
//	var wg sync.WaitGroup
//	fetches.EachRecord(func(record *kgo.Record) {
//		wg.Add(1)
//		var ackOnce sync.Once
//		ack := func() {
//			ackOnce.Do(func() {
//				wg.Done()
//			})
//		}
//		var failOnce sync.Once
//		fail := func() {
//			failOnce.Do(func() {
//				if err := PublishRecord(ctx, client, topic + "-dlq", record); err == nil {
//					// call ack now to signal we have finished processing this record
//					ack()
//				} else {
//					// if there is an error, we should not ack. The consumption is now stuck.
//					errs <- fmt.Errorf("failed to publish message to dlq %s-dql: %w", topic, err)
//				}
//			})
//		}
//		process(record, ack, fail)
//	})
//	wg.Wait()
//	return
//}
//
////func aggregateFetchErrors(errs kgo.FetchError[]) error {
////}
//
