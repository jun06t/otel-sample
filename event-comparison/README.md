# event-comparison — span を wide event にし、残りの置き場を選び分ける

参照: [Observability 1.0 と Observability 2.0](https://christina04.hatenablog.com/entry/observability_2_0) / [All you need is Wide Events, not "Metrics, Logs and Traces"](https://isburmistrov.substack.com/p/all-you-need-is-wide-events-not-metrics) / [Semantic conventions for events](https://opentelemetry.io/docs/specs/semconv/general/events/)（Status: Development） / [Recording errors](https://opentelemetry.io/docs/specs/semconv/general/recording-errors/) / [Exceptions in logs](https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-logs/) / [Database client spans](https://opentelemetry.io/docs/specs/semconv/db/database-spans/) / [Deprecating Span Events API](https://opentelemetry.io/blog/2026/deprecating-span-events/)

「ユーザー取得 → cache miss → DB クエリを最大 3 回リトライ」という 1 つの処理で、
すべての span を **wide event**（高次元・高カーディナリティな 1 行）にし、
それ以外は記録したいものごとに置き場を選び分けるサンプル。stdout exporter で出力するので、
バックエンドなしで **何がどの信号に・どの形で載るか**を確認できる。

## span を wide event にする

span と聞くと、名前・所要時間・status と数個のタグを持つ Observability 1.0 のイメージになりがち。
Observability 2.0 の wide event では、**span 1 本を 1 行、属性を列**とみなし、
1 span あたり数十〜数百の列（dimensions）を持たせる。関係しそうな文脈は何に使うか分からなくても属性として足し、
`user.id` のような cardinality が高い値も避けない。

Honeycomb はこれを "arbitrarily wide structured event" と呼び、"dozens to hundreds of dimensions per event" を持つもの、
と説明している（[Structured Events Are the Basis of Observability](https://www.honeycomb.io/blog/structured-events-basis-observability)）。
OTel の span との関係は、ドキュメントで "attach contextual information to the spans"（span に文脈を属性として付ける）と表現している
（[Add Custom Instrumentation](https://docs.honeycomb.io/send-data/standardize/add-custom-instrumentation)）。

`go run . -fail` の 1 トレースを表にすると次のようになる（空欄はその span に無い列）。

| span.name | status | user.id | user_agent.os.version | user_agent.version | app.user.plan | cache.hit | retry.count | app.db.attempt | app.db.pool.idle | error.type | … |
|---|---|---|---|---|---|---|---|---|---|---|---|
| fetch user | Error | alice | 14 | 2.3.1 | premium | false | 2 | | | DBTimeoutError | … |
| cache get | Unset | | | | | false | | | | | … |
| SELECT users | Error | | | | | | | 1 | 0 | DBTimeoutError | … |
| SELECT users | Error | | | | | | | 2 | 0 | DBTimeoutError | … |
| SELECT users | Error | | | | | | | 3 | 0 | DBTimeoutError | … |

- **リクエスト文脈はルート span に**：`user_agent.*`（アプリと OS のバージョン）、`app.user.country`、`app.user.plan`、`app.feature.new_profile` は context に入れ、`requestInfoProcessor` がルート span の `fetch user` に付ける（後述）
- **メイン span（`fetch user`）には処理の入力と結果**：処理の入力である `user.id` は `tracer.Start` で、終わるまで分からない `cache.hit`、`retry.count`、`app.db.call.count`、`app.db.duration_ms`、`error.type` は終了時の `defer` で載せる
- **子 span にはその操作固有の文脈**：`cache get` には `cache.key` / `cache.hit`、`SELECT users` には `db.*`、`db.client.connection.pool.name`、試行番号 `app.db.attempt`、プールの状態 `app.db.pool.in_use` / `app.db.pool.idle`、成功時の `db.response.returned_rows`

これを Honeycomb や SigNoz（ClickHouse）のようなバックエンドに入れると、事前にメトリクスを定義しなくても、
どの列でも絞り込み・グループ化できる（unknown unknowns の調査）。

```
WHERE user.id = "alice" AND status = Error              -- 問い合わせの 1 件を抽出
WHERE span.name = "fetch user" GROUP BY user_agent.os.version, user_agent.version
                                                        -- 未知の組み合わせでエラー率を切る
WHERE span.name = "SELECT users" GROUP BY app.db.pool.idle
                                                        -- 子 span の列でも集計できる
```

> [!note] 集計軸にしたい列は同じ span に載せる
> 別々の span にある列は、同時に条件にできない。たとえば `WHERE span.name = "SELECT users" AND app.user.plan = "premium"` は、
> `app.user.plan` が `fetch user` にしか無いので 0 件になる。
> リクエスト文脈を子 span にも付ければ解消できるが、そのぶんデータ量が増える。このサンプルはルート span だけに付けている。

## リクエスト文脈を context で渡す

Go の `context` のドキュメントは「context の値は、プロセスや API をまたぐリクエスト単位のデータにだけ使い、
関数のオプション引数の受け渡しには使わない」としている。そこで、リクエストの情報を使い道で分けている。

| 情報 | 使い道 | 渡し方 |
|---|---|---|
| user ID | DB の検索キーとして処理に使う | 引数（`fetchUser(ctx, "alice")`）。入力が context に隠れると、関数のシグネチャから依存が読めなくなる |
| OS、アプリのバージョン、国、プラン、feature flag の評価結果 | テレメトリーの次元にしか使わない | context（`withRequestInfo`）。どの層の span にも付けたい横断的な情報だから |

- context のキーは、パッケージ間の衝突を避けるため外から見えない独自型（`requestInfoKey struct{}`）にし、`withRequestInfo` / `requestInfoFrom` からだけ触る
- context の値は変えない。`cache.hit` や `retry.count` のように処理の途中で変わる値は context に入れない
- `requestInfoProcessor` は SDK の `SpanProcessor` で、`OnStart` で context から取り出して span に付ける。各関数は requestInfo を意識しなくてよい
- `OnStart` は子 span を含むすべての span で 1 回ずつ呼ばれるが、属性を付けるのは**このサービスのルート span**（親 span がいない、または親が別プロセスにいる span）だけにしている。HTTP サーバーでは上流から trace context が伝播されてくるので、「親がいない」だけで判定すると入口の span にも付かなくなる
- HTTP サーバーなら、`withRequestInfo` は middleware で呼ぶ。User-Agent、アクセストークンのクレーム、geo ヘッダーなどから組み立てる（このサンプルは CLI なので `main.go` で直接作っている）

> [!note] sampling との関係
> sampler が判断に使えるのは span 作成時にある属性だけ。`OnStart` は sampler の判断の後に、記録される span に対してだけ呼ばれるので、
> `requestInfoProcessor` が付けた属性は head sampling には使えない（Collector の tail sampling なら使える）。
> `user.id` を `tracer.Start` で渡しているのは、作成時の属性として sampler に見せるため。

> [!note] 下流のサービスにも伝える場合
> context の値はプロセス内にしか届かない。下流のサービスの span にも同じ列を付けたいなら、OTel の Baggage に入れ、
> contrib の `baggagecopy.NewSpanProcessor(filter)` で span の属性にコピーする。
> ただし Baggage は HTTP ヘッダーで下流（外部サービスを含む）にそのまま流れるので、個人情報は入れず filter で絞る。

## 置き場の選び方

wide event 以外の置き場は、OTel の semconv（Events）の指針に従っている。
「wide event」は OTel の公式用語ではなく、この表のどこか 1 行に当たるものでもない。span にどれだけの文脈（dimensions）を持たせるかという粒度の捉え方。

| 記録したいもの | 置き場 | このサンプルでの例 |
|---|---|---|
| 区間と境界がある操作 | span | `fetch user`、子 span の `cache get`、`SELECT users` |
| 操作全体の性質で、独自の時刻が不要 | span 属性 | 各 span の wide event の列（上の表） |
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
Span "SELECT users"  parent=fetch user  CLIENT  status=Error
                     db.system.name=postgresql, db.operation.name=SELECT, db.collection.name=users,
                     db.client.connection.pool.name=users-primary, app.db.attempt=1,
                     app.db.pool.in_use=10, app.db.pool.idle=0, error.type=*main.DBTimeoutError
LogRecord            EventName=db.query.retry  WARN  Body="db query timeout"  retry.attempt=2, retry.backoff_ms=10, error.type=...
  （2 回目の SELECT users と db.query.retry も同様）
Span "SELECT users"  （3 回目）status=Error
LogRecord            EventName=user.fetch.exception  ERROR  Body="db query timeout"  exception.type=..., exception.message=..., exception.stacktrace=...
Span "fetch user"    status=Error
                     user.id=alice, user_agent.name=ExampleApp, user_agent.version=2.3.1,
                     user_agent.os.name=Android, user_agent.os.version=14,
                     app.user.country=JP, app.user.plan=premium, app.feature.new_profile=true,
                     cache.hit=false, retry.count=2, app.db.call.count=3, app.db.duration_ms=63,
                     error.type=*main.DBTimeoutError
```

- 診断ログは `SELECT users` の span に、event は `fetch user` の span に、trace_id / span_id で紐づく
- 試行ごとの失敗は `SELECT users` span の status と `error.type` に残る。`fetch user` の status は、最終的に失敗したときだけ `Error` にする（[Recording errors](https://opentelemetry.io/docs/specs/semconv/general/recording-errors/)）
- DB の試行は、DB client span の規約どおり 1 回の呼び出しごとに CLIENT span にしている。規約が「1 つの span にまとめる」としているのは DB クライアントの内部で行うリトライで、このサンプルのリトライはアプリ側で行っている

`-span-events` の場合、event だけが LogRecord ではなく `fetch user` span の Events に入る。

```
Span "fetch user"  status=Error  user.id=alice, ...（wide event の属性は同じ）
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
- **文脈は span に全部載せる**：何に使うか分からなくても載せておくと、後から任意の組み合わせで絞り込める。`app.db.call.count` のように `retry.count` から計算できる値も、そのまま集計に使えるよう重ねて載せる
- **診断の数値は span 属性にも載せる**：プールの状態は普通のログのメッセージにも入れているが、集計に使えるよう `SELECT users` span の列にもしている
- **診断メッセージは普通のログ**：名前で引く想定がないので EventName を付けない
- **エラーメッセージは Body に入れる**：`error.message` 属性は非推奨。log-based event の識別も、非推奨の `event.name` 属性ではなく EventName フィールドで行う

## 属性について

| 属性 | 種類 |
|---|---|
| `error.type`, `exception.*`, `user.id`, `user_agent.*`, `db.system.name`, `db.operation.name`, `db.collection.name`, `db.client.connection.pool.name`, `db.response.returned_rows` | semconv の標準属性（`user_agent.*` や `db.client.connection.pool.name` などは Development） |
| `app.*`, `cache.key`, `cache.hit`, `retry.count`, `retry.attempt`, `retry.backoff_ms` | このサンプル独自の属性 |

国やプランのように semconv に適切な名前がない文脈は、将来の semconv と衝突しないよう `app.` を付けている。

`error.type` には Go の型名（`%T`）を使い、`RecordError` が付ける `exception.type` とそろえている。
`retry.attempt` は、これから行う試行の番号（2 回目なら 2）。

OTLP でバックエンドに送って span と LogRecord の相関を画面で見る例は [`../signoz-wide-event`](../signoz-wide-event) を参照。

## ファイル構成

```
event-comparison/
├── main.go       # -fail / -span-events フラグ、リクエスト文脈の組み立て、終了コード
├── telemetry.go  # stdout exporter の TracerProvider / LoggerProvider
├── reqctx.go     # リクエスト文脈の context と、全 span に付ける SpanProcessor
├── fetch.go      # fetchUser：メイン span の入力と結果・リトライ・例外の置き場
├── store.go      # cache / DB の子 span（操作固有の文脈）と、診断用の普通のログ
└── events.go     # event を Logs API / span event で書く 2 つの実装
```
