# event-comparison — wide event / span event / log-based event を同じシナリオで比べる

参照: [Deprecating Span Events API](https://opentelemetry.io/blog/2026/deprecating-span-events/)（2026-03） / [OTEP 4430](https://github.com/open-telemetry/opentelemetry-specification/blob/main/oteps/4430-span-event-api-deprecation-plan.md)

「ユーザー取得 → cache miss → DB クエリを最大 3 回リトライ」という同じ処理を、
3 通りの置き場で記録するサンプル。stdout exporter で出力するので、バックエンドなしで
**どの信号に・どの形で載るか**をそのまま見比べられる。

| style | 置き場 | 信号 | 実装 |
|---|---|---|---|
| `wide` | メイン span の属性に回数として集約 | traces | [`wide.go`](wide.go) |
| `span` | 親 span の Events に時点の注釈 | traces | [`span.go`](span.go) |
| `log` | `event.name` 付き LogRecord（trace_id / span_id で紐づく） | logs | [`logevent.go`](logevent.go) |

> [!note] Span Event API の deprecation
> 仕様として deprecated になったのは `AddEvent` / `RecordException`（Go では `RecordError`）の **API** で、
> wide event（span 属性）は関係ない。なお Go SDK v1.44 の時点では、これらのメソッドに `Deprecated:` 表記はまだ付いていない。

## 実行

```bash
go run . -style=wide        # 1 回 retry して成功
go run . -style=span -fail  # 2 回 retry して失敗
go run . -style=log -fail
```

| フラグ | 既定 | 説明 |
|---|---|---|
| `-style` | `wide` | `wide` / `span` / `log` |
| `-fail` | `false` | DB クエリをリトライ上限まで失敗させる |

traces / logs とも同期 export（`WithSyncer` / `SimpleProcessor`）にしているので、
LogRecord は Emit した瞬間に、span は End した瞬間に出力される。

## 出力の違い（`-fail` の場合、要点のみ）

### wide

span 1 本。途中経過は「いつ起きたか」ではなく「何回起きたか」として属性に残る。

```
Span "GET /users/{id}"  status=Error
  Attributes: user.id=alice, cache.misses=1, retry.count=2, error.type=*main.DBTimeoutError
```

### span

span 1 本。その `Events` 配列に時点の注釈が並ぶ。`RecordError` は `exception` という名前の span event になる。

```
Span "GET /users/{id}"  status=Error
  Attributes: user.id=alice
  Events:
    cache.miss  cache.key=user:alice
    retry       retry.attempt=1, error.message=...
    retry       retry.attempt=2, error.message=...
    exception   exception.type=*main.DBTimeoutError, exception.message=...
```

### log

LogRecord が先に出て、最後に span が出る。LogRecord は span と同じ TraceID / SpanID を持つ。
エラーは `SetStatus` で失敗の事実を span に、中身を ERROR ログ（`event.name` なし）に分けている。

```
LogRecord  EventName=cache.miss  Severity=INFO   cache.key=user:alice
LogRecord  EventName=retry       Severity=WARN   retry.attempt=1, error.message=...
LogRecord  EventName=retry       Severity=WARN   retry.attempt=2, error.message=...
LogRecord  (EventName なし)       Severity=ERROR  exception.type=..., exception.message=...
Span "GET /users/{id}"  status=Error  Attributes: user.id=alice
```

普通のログと log-based event はどちらも logs 信号で、違いは `EventName` の有無だけ。

## 使い分け

| 記録したいもの | 置き場 |
|---|---|
| リクエストの文脈（user.id、回数、フラグ） | メイン span の属性（wide） |
| 失敗の事実 | `span.SetStatus(codes.Error, …)` |
| どうしても要る時点のピン（retry など） | log-based event |
| エラーの中身 | severity=ERROR の LogRecord |

基本は wide にまとめ、時点が必要なときだけ log-based event を足す。
OTLP でバックエンドに送って span と LogRecord の相関を画面で見る例は [`../signoz-wide-event`](../signoz-wide-event) を参照。

## ファイル構成

```
event-comparison/
├── main.go       # -style / -fail フラグ
├── telemetry.go  # stdout exporter の TracerProvider / LoggerProvider
├── scenario.go   # cache / DB を模した共通処理
├── wide.go       # wide event
├── span.go       # span event
└── logevent.go   # log-based event
```
