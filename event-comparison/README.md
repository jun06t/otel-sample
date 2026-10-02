# event-comparison — span / span 属性 / event / 普通のログの置き場を選び分ける

参照: [Semantic conventions for events](https://opentelemetry.io/docs/specs/semconv/general/events/)（Status: Development） / [Recording errors](https://opentelemetry.io/docs/specs/semconv/general/recording-errors/) / [Exceptions in logs](https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-logs/) / [Database client spans](https://opentelemetry.io/docs/specs/semconv/db/database-spans/) / [Deprecating Span Events API](https://opentelemetry.io/blog/2026/deprecating-span-events/)

「ユーザー取得 → cache miss → DB クエリを最大 3 回リトライ」という 1 つの処理で、
記録したいものごとに置き場を選び分けるサンプル。stdout exporter で出力するので、
バックエンドなしで **何がどの信号に・どの形で載るか**を確認できる。

## 置き場の選び方

OTel の semconv（Events）の指針に従っている。「wide event」という用語は OTel の公式用語ではなく、
ここでは「操作全体の性質を span 属性に集約する」ことに当たる。

| 記録したいもの | 置き場 | このサンプルでの例 |
|---|---|---|
| 区間と境界がある操作 | span | `fetch user`、子 span の `cache get`、`SELECT users` |
| 操作全体の性質で、独自の時刻が不要 | span 属性 | `user.id`、`cache.hit`、`retry.count`、`error.type` |
| 名前の付いた時点の出来事（0 回以上起き、その回の時刻・severity・属性が要る） | event（EventName 付き LogRecord） | `db.query.retry`、`user.fetch.exception` |
| 名前で引かない診断メッセージ | 普通の LogRecord（EventName なし） | `connection pool exhausted: ...` |

span event と log-based event は置き場の選択肢ではなく、**同じ event を書く API の違い**。
既定は Logs API で書き、`-span-events` を付けると従来の `AddEvent` / `RecordError` で書く。

## 実行

```bash
go run .                      # 1 回 retry して成功（終了コード 0）
go run . -fail                # 2 回 retry して失敗（終了コード 1）
go run . -fail -span-events   # 同じ event を span event で記録する
```

| フラグ | 既定 | 説明 |
|---|---|---|
| `-fail` | `false` | DB クエリをリトライ上限まで失敗させる |
| `-span-events` | `false` | event を Logs API ではなく span event（`AddEvent` / `RecordError`）で記録する |

traces / logs とも同期 export（`WithSyncer` / `SimpleProcessor`）にしているので、
LogRecord は Emit した瞬間に、span は End した瞬間に出力される。

## 出力（`-fail` の場合、要点のみ）

```
Span "cache get"     parent=fetch user  cache.key=user:alice, cache.hit=false
LogRecord            (EventName なし)  DEBUG  Body="connection pool exhausted: in_use=10 idle=0"
Span "SELECT users"  parent=fetch user  CLIENT  status=Error  db.system.name=postgresql, ..., error.type=*main.DBTimeoutError
LogRecord            EventName=db.query.retry  WARN  Body="db query timeout"  retry.attempt=2, retry.backoff_ms=10, error.type=...
  （2 回目の SELECT users と db.query.retry も同様）
Span "SELECT users"  （3 回目）status=Error
LogRecord            EventName=user.fetch.exception  ERROR  Body="db query timeout"  exception.type=..., exception.message=..., exception.stacktrace=...
Span "fetch user"    status=Error  user.id=alice, cache.hit=false, retry.count=2, error.type=*main.DBTimeoutError
```

- 診断ログは `SELECT users` の span に、event は `fetch user` の span に、trace_id / span_id で紐づく
- 試行ごとの失敗は `SELECT users` span の status と `error.type` に残る。`fetch user` の status は、最終的に失敗したときだけ `Error` にする（[Recording errors](https://opentelemetry.io/docs/specs/semconv/general/recording-errors/)）
- DB の試行は、DB client span の規約どおり 1 回の呼び出しごとに CLIENT span にしている。規約が「1 つの span にまとめる」としているのは DB クライアントの内部で行うリトライで、このサンプルのリトライはアプリ側で行っている

`-span-events` の場合、event だけが LogRecord ではなく `fetch user` span の Events に入る。

```
Span "fetch user"  status=Error  user.id=alice, cache.hit=false, retry.count=2, error.type=*main.DBTimeoutError
  Events:
    db.query.retry  retry.attempt=2, retry.backoff_ms=10, error.type=...
    db.query.retry  retry.attempt=3, retry.backoff_ms=20, error.type=...
    exception       exception.type=..., exception.message=..., exception.stacktrace=...
```

span event には severity も Body もないので、retry の severity とメッセージは記録されない。
例外の名前も `exception` 固定で、`user.fetch.exception` のような操作名は付けられない。

> [!note] Span Event API の非推奨化
> 2026-03 に、`AddEvent` / `RecordException`（Go では `RecordError`）の API を段階的に非推奨にする方針が発表され、
> 新しいイベントと例外には Logs API が推奨されている。現行の Trace API 仕様にはまだ残っており、
> Go SDK v1.45 の時点では、これらのメソッドに `Deprecated:` 表記はまだ付いていない。

## なぜこの置き場か

- **`cache.hit` は span 属性**：cache を引くのは 1 回だけで、独自の時刻も要らない。区間は `cache get` 子 span が持つ
- **`retry.count` は span 属性、`db.query.retry` は event**：何回リトライしたかは操作全体の性質。一方、各リトライの判断は 0 回以上起き、その回の待ち時間（`retry.backoff_ms`）を持つ時点の出来事
- **例外は event**：semconv に従い、EventName を「操作名 + `.exception`」にする。失敗の事実は span の status と `error.type` に分ける
- **`user.id` は span 開始時に渡す**：開始時に分かる属性は `tracer.Start` で渡す。sampler が判断に使えるのは span 作成時にある属性だけなので、終了時に書く属性は sampling に効かない
- **診断メッセージは普通のログ**：名前で引く想定がないので EventName を付けない
- **エラーメッセージは Body に入れる**：`error.message` 属性は非推奨。log-based event の識別も、非推奨の `event.name` 属性ではなく EventName フィールドで行う

## 属性について

| 属性 | 種類 |
|---|---|
| `error.type`, `exception.*`, `user.id`, `db.system.name`, `db.operation.name`, `db.collection.name` | semconv の標準属性 |
| `cache.key`, `cache.hit`, `retry.count`, `retry.attempt`, `retry.backoff_ms` | このサンプル独自の属性 |

`error.type` には Go の型名（`%T`）を使い、`RecordError` が付ける `exception.type` とそろえている。
`retry.attempt` は、これから行う試行の番号（2 回目なら 2）。

OTLP でバックエンドに送って span と LogRecord の相関を画面で見る例は [`../signoz-wide-event`](../signoz-wide-event) を参照。

## ファイル構成

```
event-comparison/
├── main.go       # -fail / -span-events フラグ、終了コード
├── telemetry.go  # stdout exporter の TracerProvider / LoggerProvider
├── fetch.go      # fetchUser：span 属性・リトライ・例外の置き場
├── store.go      # cache / DB の子 span と、診断用の普通のログ
└── events.go     # event を Logs API / span event で書く 2 つの実装
```
