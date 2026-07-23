package main

import "github.com/kelseyhightower/envconfig"

// Config は span-link デモの設定。環境変数から読み込む。
type Config struct {
	ServiceName string `envconfig:"SERVICE_NAME" default:"example-span-link"`
	// ExporterEndpoint が空なら stdout、値があれば OTLP gRPC でその宛先へ送信。
	ExporterEndpoint string `envconfig:"EXPORTER_ENDPOINT" default:""`

	// Pub/Sub 関連。
	ProjectID          string `envconfig:"PROJECT_ID" default:"demo-project"`
	TopicID            string `envconfig:"TOPIC_ID" default:"demo-topic"`
	SubscriptionID     string `envconfig:"SUBSCRIPTION_ID" default:"demo-sub"`
	PubsubEmulatorHost string `envconfig:"PUBSUB_EMULATOR_HOST" default:""`

	// BatchSize は 1 回の process span でまとめて処理するメッセージ数。
	// この数だけの span link が 1 本の process span に張られる。
	BatchSize int `envconfig:"BATCH_SIZE" default:"3"`
}

func LoadConfig() (*Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, err
	}
	return &c, nil
}
