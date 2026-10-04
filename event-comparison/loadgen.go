package main

import (
	"fmt"
	"math/rand/v2"
)

// scenario は 1 リクエストぶんの条件。cache と DB の振る舞いもここで決める。
//
// アプリのコード(userService など)はリクエスト文脈を処理に使わない。
// どの条件で失敗しやすいかは、この負荷生成側だけが知っている。
type scenario struct {
	userID     string
	info       requestInfo
	cacheHit   bool
	dbTimeouts int // 最初の何回の DB 呼び出しをタイムアウトさせるか
}

// fixedScenario は -requests 1 のときの、毎回同じ 1 リクエスト。
// cache は miss し、DB は 1 回目だけタイムアウトしてリトライで成功する。fail なら全試行が失敗する。
func fixedScenario(fail bool) scenario {
	sc := scenario{
		userID: "alice",
		info: requestInfo{
			Country:    "JP",
			Plan:       "premium",
			OSName:     "Android",
			OSVersion:  "14",
			AppName:    "ExampleApp",
			AppVersion: "2.3.1",
			NewProfile: true,
		},
		dbTimeouts: 1,
	}
	if fail {
		sc.dbTimeouts = maxAttempts
	}
	return sc
}

// randomScenario は、文脈をばらつかせたリクエストを作る。
//
// 仕込んだ不具合: Android 14 × アプリ 2.3.1 の組み合わせだけ DB がタイムアウトしやすい。
// SigNoz で fetch user のエラーを OS のバージョンやアプリのバージョンで GROUP BY し、
// 事前に知らない組み合わせ(unknown unknowns)にたどり着けるかを試すためのもの。
func randomScenario(r *rand.Rand, fail bool) scenario {
	osName, osVersions := "iOS", []string{"17", "18"}
	if r.IntN(2) == 0 {
		osName, osVersions = "Android", []string{"13", "14"}
	}
	info := requestInfo{
		Country:    pick(r, "JP", "US", "DE", "BR", "IN"),
		Plan:       pick(r, "free", "free", "free", "premium", "premium", "enterprise"),
		OSName:     osName,
		OSVersion:  pick(r, osVersions...),
		AppName:    "ExampleApp",
		AppVersion: pick(r, "2.3.0", "2.3.1", "2.4.0"),
		NewProfile: r.IntN(2) == 0,
	}

	sc := scenario{
		userID:   fmt.Sprintf("user-%03d", r.IntN(200)),
		info:     info,
		cacheHit: r.IntN(10) < 3, // 30% は cache hit
	}

	if !sc.cacheHit {
		p := r.IntN(100)
		switch {
		case fail:
			sc.dbTimeouts = maxAttempts
		case info.OSName == "Android" && info.OSVersion == "14" && info.AppVersion == "2.3.1":
			// 70% は全試行がタイムアウトして失敗、残りは 1 回目だけタイムアウト
			if p < 70 {
				sc.dbTimeouts = maxAttempts
			} else {
				sc.dbTimeouts = 1
			}
		case p < 3:
			sc.dbTimeouts = maxAttempts
		case p < 15:
			sc.dbTimeouts = 1
		}
	}
	return sc
}

func pick[T any](r *rand.Rand, xs ...T) T {
	return xs[r.IntN(len(xs))]
}
