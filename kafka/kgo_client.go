package kafka

import (
	//"github.com/emillamm/envx"
	"github.com/twmb/franz-go/pkg/kgo"
)

func LoadKgoClient(
	consumeTopics []string,
	group string,
) (client *kgo.Client, err error) {

	// TODO support other connection schemes
	opts := []kgo.Opt{
		kgo.SeedBrokers("localhost:29092"),
		kgo.AllowAutoTopicCreation(),
	}

	// add topics
	for _, t := range consumeTopics {
		opts = append(opts, kgo.ConsumeTopics(t))
	}

	// add group
	if group != "" {
		opts = append(opts, kgo.ConsumerGroup(group))
	}

	client, err = kgo.NewClient(opts...)
	return
}

