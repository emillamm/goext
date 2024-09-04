package kafka

//import (
//	"testing"
//	//"time"
//	"github.com/twmb/franz-go/pkg/kgo"
//	"context"
//)
//
//func TestPoc(t *testing.T) {
//	//t.Run("", func(t *testing.T) {
//	//})
//
//	t.Run("publish and consume", func(t *testing.T) {
//		ctx := context.Background()
//		opts := []kgo.Opt{
//			kgo.SeedBrokers("localhost:29092"),
//			kgo.ConsumerGroup("test-group"),
//			kgo.ConsumeTopics("test-topic"),
//			kgo.AllowAutoTopicCreation(),
//		}
//
//		println(1)
//		cl, err := kgo.NewClient(opts...)
//		if err != nil {
//			t.Fatal(err)
//		}
//
//		if err := PublishRecord(ctx, cl, "test-topic", &kgo.Record{Topic: "test-topic", Value: []byte("bar")}); err != nil {
//			t.Fatal(err)
//		}
//		cl.Close()
//	})
//
//	//t.Run("test kafka", func(t *testing.T) {
//
//	//	t.Log("starting")
//	//	opts := []kgo.Opt{
//	//		kgo.SeedBrokers("http://localhost:9092"),
//	//		kgo.ConsumerGroup("test-group-2"),
//	//		kgo.ConsumeTopics("test-topic"),
//	//	}
//
//	//	cl, err := kgo.NewClient(opts...)
//	//	if err != nil {
//	//		t.Fatal(err)
//	//	}
//
//	//	go func () {
//	//		t.Log("starting consuming")
//	//		for {
//	//			fetches := cl.PollFetches(context.Background())
//	//			if fetches.IsClientClosed() {
//	//				t.Log("client closed")
//	//				return
//	//			}
//	//			fetches.EachError(func(s string, p int32, err error) {
//	//				t.Errorf("fetch err topic %s partition %d: %v", s, p, err)
//	//			})
//	//			fetches.EachRecord(func(r *kgo.Record) {
//	//				t.Log("receiving record")
//	//				t.Logf("record: %#v", r)
//	//			})
//	//			if err := cl.CommitUncommittedOffsets(context.Background()); err != nil {
//	//				t.Errorf("commit records failed: %v", err)
//	//				continue
//	//			}
//	//		}
//	//	}()
//
//	//	time.Sleep(2 * time.Second)
//	//	cl.Close()
//	//	time.Sleep(1 * time.Second)
//
//	//})
//}
//
