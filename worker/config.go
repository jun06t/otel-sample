package main

import "github.com/kelseyhightower/envconfig"

// Config は常駐ワーカーの設定。環境変数から読み込む。
type Config struct {
	ServiceName string `envconfig:"SERVICE_NAME" default:"example-worker"`
	// ExporterEndpoint が空なら stdout、値があれば OTLP gRPC でその宛先へ送信。
	ExporterEndpoint string `envconfig:"EXPORTER_ENDPOINT" default:""`

	// Pub/Sub 関連。
	ProjectID      string `envconfig:"PROJECT_ID" default:"demo-project"`
	TopicID        string `envconfig:"TOPIC_ID" default:"demo-topic"`
	SubscriptionID string `envconfig:"SUBSCRIPTION_ID" default:"demo-sub"`
	// PubsubEmulatorHost を設定すると Pub/Sub クライアントは自動でエミュレータに接続する
	// （ライブラリが PUBSUB_EMULATOR_HOST 環境変数を参照する）。
	PubsubEmulatorHost string `envconfig:"PUBSUB_EMULATOR_HOST" default:""`
	// Bootstrap が true のとき、起動時に topic/subscription を作成しテストメッセージを publish する（デモ用）。
	Bootstrap bool `envconfig:"BOOTSTRAP" default:"true"`
}

func LoadConfig() (*Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, err
	}
	return &c, nil
}
