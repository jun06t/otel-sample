package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// wideEvent は途中経過をローカル変数に溜め、終了時にまとめて span 属性へ書く。
//
// 出力は span 1 本だけ。cache miss や retry は「いつ起きたか」ではなく
// 「何回起きたか」として属性に残り、属性で slice & dice できる。
func wideEvent(ctx context.Context, userID string, fail bool) (err error) {
	ctx, span := tracer.Start(ctx, "GET /users/{id}")
	defer span.End()

	var cacheMisses, retries int

	// span.End より先に実行される(defer は LIFO)。
	defer func() {
		span.SetAttributes(
			attribute.String("user.id", userID),
			attribute.Int("cache.misses", cacheMisses),
			attribute.Int("retry.count", retries),
		)
		if err != nil {
			// 失敗の事実は status と error.type で表す。
			span.SetStatus(codes.Error, "fetch user failed")
			span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		}
	}()

	if _, ok := cacheGet(ctx, cacheKey(userID)); !ok {
		cacheMisses++
	}

	for attempt := 1; ; attempt++ {
		_, err = dbQuery(ctx, userID, attempt, fail)
		if err == nil || attempt == maxAttempts {
			return err
		}
		retries++
	}
}
