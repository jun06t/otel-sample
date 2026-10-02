package main

import (
	"context"
	"fmt"
	"time"
)

// spanName はユーザー取得処理を表す span 名。
// HTTP server span ではないので、HTTP の semconv に従った名前にはしない。
const spanName = "fetch user"

// maxAttempts は DB クエリの最大試行回数。
const maxAttempts = 3

// DBTimeoutError は DB クエリのタイムアウトを表す。
type DBTimeoutError struct {
	Attempt int
}

func (e *DBTimeoutError) Error() string {
	return fmt.Sprintf("db query timeout (attempt %d)", e.Attempt)
}

// errorType は error.type / exception.type に入れる値を返す。
// span.RecordError が付ける exception.type と同じく、Go の型名を使う。
func errorType(err error) string {
	return fmt.Sprintf("%T", err)
}

func cacheKey(userID string) string {
	return "user:" + userID
}

// cacheGet は常に miss する cache を模す。
func cacheGet(_ context.Context, _ string) (string, bool) {
	time.Sleep(5 * time.Millisecond)
	return "", false
}

// dbQuery は DB からユーザーを引く処理を模す。
// 1 回目は必ずタイムアウトし、fail=true なら全試行がタイムアウトする。
func dbQuery(_ context.Context, userID string, attempt int, fail bool) (string, error) {
	time.Sleep(20 * time.Millisecond)
	if attempt == 1 || fail {
		return "", &DBTimeoutError{Attempt: attempt}
	}
	return "name-of-" + userID, nil
}
