package main

import (
	"context"
	"fmt"
	"time"
)

// maxAttempts は DB クエリの最大試行回数。
const maxAttempts = 3

// DBTimeoutError は DB クエリのタイムアウトを表す。
type DBTimeoutError struct {
	Attempt int
}

func (e *DBTimeoutError) Error() string {
	return fmt.Sprintf("db query timeout (attempt %d)", e.Attempt)
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
