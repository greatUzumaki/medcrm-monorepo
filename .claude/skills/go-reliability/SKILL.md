---
name: go-reliability
description: Use for performance and stability work in the Go backend — server/DB/HTTP timeouts, pgx pool sizing, graceful shutdown, backpressure and rate limits, River workers, caching, profiling (pprof), benchmarks and k6 load tests, observability.
---

# Go: скорость и стабильность под нагрузкой

Принцип: **ограничить всё** (время, память, параллелизм, размер ввода) и **деградировать предсказуемо**, а не падать.

## HTTP-сервер (`cmd/api/main.go`)
```go
srv := &http.Server{
	Addr:              cfg.Addr,
	Handler:           h,
	ReadHeaderTimeout: 5 * time.Second,   // Slowloris
	ReadTimeout:       15 * time.Second,
	WriteTimeout:      30 * time.Second,   // SSE-роуты переопределяют через http.ResponseController
	IdleTimeout:       120 * time.Second,
	MaxHeaderBytes:    1 << 20,
}
```
- Тело запроса: `http.MaxBytesReader` (JSON 1 MB, загрузки — по роуту).
- Middleware-цепочка: recover → request_id → otel → access-log (без ПДн) → rate limit → auth → timeout (`http.TimeoutHandler` или ctx-deadline per route).
- Graceful shutdown: `signal.NotifyContext(SIGTERM)` → `/readyz` начинает отдавать 503 → `srv.Shutdown(ctx 25s)` → остановка River → `pool.Close()`.
- `/livez` (процесс жив) и `/readyz` (БД доступна) раздельно.

## Postgres (pgx)
- `pgxpool`: `MaxConns` ≈ ядра БД × 2–4 / кол-во инстансов; `MaxConnLifetime` 30–60m; `MaxConnIdleTime` 5m. Перед API — PgBouncer в transaction mode, если инстансов много (тогда `QueryExecModeCacheDescribe`, не prepared statements на уровне сессии).
- `statement_timeout` на роль приложения (например 5s), `idle_in_transaction_session_timeout` 10s.
- Транзакции короткие: никаких HTTP/LLM-вызовов внутри tx.
- N+1 запрещён: батч-запросы (`= ANY($1)`), `pgx.Batch`.
- `EXPLAIN (ANALYZE, BUFFERS)` для каждого нового запроса на горячем пути.

## Внешние вызовы (LLM, SMS, платежи)
- Один `http.Client` на интеграцию с `Timeout` и настроенным `Transport` (`MaxIdleConnsPerHost`).
- Ретраи только идемпотентных операций, экспоненциальный backoff + jitter, лимит попыток.
- Circuit breaker (sony/gobreaker) на нестабильных провайдерах; ограничение параллелизма семафором (`golang.org/x/sync/semaphore`).
- Долгое (транскрибация, эмбеддинги, рассылки) — **только в River**, не в запросе. River: `MaxWorkers` на очередь, уникальные джобы, `InsertTx` в той же транзакции, что и бизнес-изменение (outbox бесплатно).

## Конкурентность
- Каждая горутина имеет владельца, выход по `ctx.Done()`, ошибки через `errgroup`.
- Никаких неограниченных `go func()` на запрос. Fan-out — `errgroup.SetLimit`.
- `go test -race` всегда в CI.

## Rate limiting
Per-IP на публичные/auth эндпоинты, per-user и per-clinic на API, отдельно жёсткие лимиты на ИИ. Хранение счётчиков — Postgres/Redis при нескольких инстансах. Ответ `429` + `Retry-After`.

## Кэш
Только неперсональные данные (справочники, настройки клиники, прайс) — in-memory с TTL / `singleflight`. ПДн не кэшировать вне БД. HTTP: `Cache-Control: no-store` на всех ответах с ПДн.

## Наблюдаемость
- OTel трейсы (HTTP, pgx через `otelpgx`, внешние вызовы), атрибуты без ПДн.
- Prometheus: RED-метрики по роутам (шаблон роута, не реальный путь), размер пула, очередь River, токены LLM.
- `net/http/pprof` только на отдельном внутреннем порту.

## Проверка производительности
- Бенчмарк горячей функции: `go test -bench . -benchmem`, сравнение `benchstat`.
- k6 сценарии в `apps/api/load/`: расписание дня клиники, поиск пациента, создание записи. Цели (стартовые): p95 < 150ms, p99 < 400ms, 0 ошибок 5xx при целевом RPS, деградация через 429, а не через рост латентности.
- Профиль под нагрузкой: `go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30`.
