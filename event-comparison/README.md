# event-comparison — wide event / span event / log-based event を同じシナリオで比べる

参照: [Deprecating Span Events API](https://opentelemetry.io/blog/2026/deprecating-span-events/)（2026-03） / [OTEP 4430](https://github.com/open-telemetry/opentelemetry-specification/blob/main/oteps/4430-span-event-api-deprecation-plan.md) / [Recording errors](https://opentelemetry.io/docs/specs/semconv/general/recording-errors/) / [Exceptions in logs](https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-logs/)

「ユーザー取得 → cache miss → DB クエリを最大 3 回リトライ」という同じ処理を、
3 通りの置き場で記録するサンプル。stdout exporter で出力するので、バックエンドなしで
**どの信号に・どの形で載るか**をそのまま見比べられる。

| style | 置き場 | 信号 | 実装 |
|---|---|---|---|
| `wide` | メイン span の属性に回数として集約 | traces | [`wide.go`](wide.go) |
| `span` | 親 span の Events に時点の注釈 | traces | [`span.go`](span.go) |
| `log` | EventName 付きの LogRecord（trace_id / span_id で紐づく） | logs | [`logevent.go`](logevent.go) |

> [!note] Span Event API の非推奨化
> 2026-03 に、`AddEvent` / `RecordException`（Go では `RecordError`）の **API** を段階的に非推奨にする方針が発表され、
> 新しいイベントと例外には Logs API が推奨されている。現行の Trace API 仕様にはまだ残っており、
> Go SDK v1.45 の時点では、これらのメソッドに `Deprecated:` 表記はまだ付いていない。
> wide event（span 属性）はこの非推奨化と関係ない。

> [!note] EventName と `event.name` 属性
> log-based event は、LogRecord の **EventName フィールド**が空でないもの。
> かつての `event.name` **属性**は非推奨で、Go では `Record.SetEventName` で EventName フィールドに設定する。

## 実行

```bash
go run . -style=wide        # 1 回 retry して成功（終了コード 0）
go run . -style=span -fail  # 2 回 retry して失敗（終了コード 1）
go run . -style=log -fail
```

| フラグ | 既定 | 説明 |
|---|---|---|
| `-style` | `wide` | `wide` / `span` / `log` |
| `-fail` | `false` | DB クエリをリトライ上限まで失敗させる |

traces / logs とも同期 export（`WithSyncer` / `SimpleProcessor`）にしているので、
LogRecord は Emit した瞬間に、span は End した瞬間に出力される。

## 3 スタイル共通のルール

[Recording errors](https://opentelemetry.io/docs/specs/semconv/general/recording-errors/) に従っている。

- 最終的に失敗したら、span の status を `Error` にし、span 自体に `error.type` を付ける
- リトライで回復しうる途中のエラーでは、span の status を変えない
- span 名は `fetch user`。HTTP server span ではないので `GET /users/{id}` のような HTTP 形式にはしない

## 出力の違い（`-fail` の場合、要点のみ）

### wide

span 1 本。途中経過は「いつ起きたか」ではなく「何回起きたか」として属性に残る。

```
Span "fetch user"  status=Error
  Attributes: user.id=alice, cache.misses=1, retry.count=2, error.type=*main.DBTimeoutError
```

### span

span 1 本。その `Events` 配列に時点の注釈が並ぶ。`RecordError` は `exception` という名前の span event になる。

```
Span "fetch user"  status=Error
  Attributes: user.id=alice, error.type=*main.DBTimeoutError
  Events:
    cache.miss      cache.key=user:alice
    db.query.retry  retry.attempt=1, error.type=*main.DBTimeoutError
    db.query.retry  retry.attempt=2, error.type=*main.DBTimeoutError
    exception       exception.type=..., exception.message=..., exception.stacktrace=...
```

### log

LogRecord が先に出て、最後に span が出る。LogRecord は span と同じ TraceID / SpanID を持つ。
例外の LogRecord は [Exceptions in logs](https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-logs/) に従い、
EventName を「操作名 + `.exception`」にしている。

```
LogRecord  EventName=cache.miss            Severity=INFO   cache.key=user:alice
LogRecord  EventName=db.query.retry        Severity=WARN   Body="db query timeout (attempt 1)"  retry.attempt=1, error.type=...
LogRecord  EventName=db.query.retry        Severity=WARN   Body="db query timeout (attempt 2)"  retry.attempt=2, error.type=...
LogRecord  EventName=user.fetch.exception  Severity=ERROR  Body="db query timeout (attempt 3)"  exception.type=..., exception.message=..., exception.stacktrace=...
Span "fetch user"  status=Error  Attributes: user.id=alice, error.type=*main.DBTimeoutError
```

普通のログと log-based event はどちらも logs 信号で、違いは EventName が空かどうかだけ。

## 属性について

| 属性 | 種類 |
|---|---|
| `error.type`, `exception.type`, `exception.message`, `exception.stacktrace`, `user.id` | semconv の標準属性 |
| `cache.key`, `cache.misses`, `retry.count`, `retry.attempt` | このサンプル独自の属性 |

エラーメッセージは、非推奨の `error.message` 属性ではなく、LogRecord の Body に入れている。
`error.type` には Go の型名（`%T`）を使い、`RecordError` が付ける `exception.type` とそろえている。

## 使い分け

| 記録したいもの | 置き場 |
|---|---|
| リクエストの文脈（user.id、回数、フラグ） | メイン span の属性（wide） |
| 失敗の事実 | `span.SetStatus(codes.Error, …)` と span の `error.type` |
| どうしても要る時点のピン（retry など） | log-based event |
| エラーの中身 | `<操作名>.exception` の LogRecord |

基本は wide にまとめ、時点が必要なときだけ log-based event を足す。
OTLP でバックエンドに送って span と LogRecord の相関を画面で見る例は [`../signoz-wide-event`](../signoz-wide-event) を参照。

## ファイル構成

```
event-comparison/
├── main.go       # -style / -fail フラグ、終了コード
├── telemetry.go  # stdout exporter の TracerProvider / LoggerProvider
├── scenario.go   # cache / DB を模した共通処理
├── wide.go       # wide event
├── span.go       # span event
└── logevent.go   # log-based event
```
