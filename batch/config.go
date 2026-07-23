package main

import "github.com/kelseyhightower/envconfig"

// Config はバッチの実行設定。環境変数から読み込む。
type Config struct {
	// ServiceName は Trace 上のサービス名。
	ServiceName string `envconfig:"SERVICE_NAME" default:"example-batch"`
	// ExporterEndpoint が空なら stdout に span を出力（そのまま go run で動く）。
	// 値があれば OTLP gRPC でその宛先（例: jaeger:4317）へ送信する。
	ExporterEndpoint string `envconfig:"EXPORTER_ENDPOINT" default:""`
}

func LoadConfig() (*Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, err
	}
	return &c, nil
}
