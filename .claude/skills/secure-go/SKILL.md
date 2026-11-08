---
name: secure-go
description: Use when writing or reviewing security-sensitive Go backend code — authentication, sessions, RBAC, input validation, crypto/keys, secrets, SQL, file uploads, outbound HTTP (SSRF), webhooks, error handling, logging redaction, security headers, dependency scanning.
---

# Безопасный Go-бэкенд

## Аутентификация и сессии (`internal/platform/auth`)
- Пароли: argon2id (`golang.org/x/crypto/argon2`, m=64MB, t=3, p=2+, соль 16B), сравнение `subtle.ConstantTimeCompare`. Проверка по списку утёкших паролей (k-anonymity / локальный список).
- Сессия: 32 байта из `crypto/rand` → base64url в cookie; в БД хранится **SHA-256** токена, не сам токен. Cookie: `HttpOnly; Secure; SameSite=Lax; Path=/; __Host-` префикс.
- Ротация токена при входе/повышении прав/смене пароля; idle 30 мин, абсолютный 12 ч; logout удаляет запись в БД.
- TOTP (RFC 6238) для персонала с доступом к медданным; recovery-коды хешированы.
- Брутфорс: лимит по IP + по логину, прогрессивная задержка, одинаковый ответ и время для «нет пользователя» / «неверный пароль».
- Сброс пароля: одноразовый токен (хеш в БД), TTL 30 мин, инвалидирует все сессии.

## CSRF
Cookie-сессия ⇒ мутирующие запросы (`POST/PUT/PATCH/DELETE`) требуют проверки `Origin`/`Sec-Fetch-Site` (allowlist доменов) **и** CSRF-токен (double-submit или синхронизатор). Go 1.25+: `http.CrossOriginProtection` как базовый слой.

## Авторизация
- Middleware: нет сессии → 401. В каждом хендлере — `authz.Require(sess, perm)`; доступ к объекту — проверка принадлежности (tenant через RLS + отношения врач/пациент в service).
- Запрещено: доверять `role`, `clinic_id`, `user_id`, `is_admin` из запроса.
- IDOR-тест на каждый эндпоинт с id в пути.

## Ввод
- Валидация по OpenAPI-схеме (oapi-codegen + `kin-openapi` request validator middleware) + доменная валидация в service.
- `http.MaxBytesReader`, `json.Decoder.DisallowUnknownFields()`.
- Нормализация перед blind index/сравнением (телефон → E.164, trim, NFC).
- SQL — только sqlc. Динамическая сортировка — через allowlist колонок, не строкой из запроса.
- Шаблоны писем/PDF — `html/template` (автоэкранирование), не `text/template`.

## Криптография (`internal/platform/crypto`)
- Только `crypto/rand`, никогда `math/rand` для токенов/ключей/ID.
- Полевое шифрование: AES-256-GCM, envelope: DEK на клинику, обёрнут KEK из KMS/Vault; формат `v<keyver>|nonce|ciphertext`; AAD = `clinic_id|table|row_id|column`.
- Ротация ключей: новая версия для записи, старые для чтения, фоновая перешифровка (River).
- HMAC-SHA256 для blind index — отдельный ключ, не тот же, что для шифрования.
- Ключи не в коде, не в репо, не в логах. Загрузка из secret store/env при старте; в памяти — в структурах без `String()`/`LogValue()`, раскрывающих значение.

## Секреты и конфиг
- Конфиг из env, валидация при старте (fail fast). `.env` не коммитится (хук + gitleaks в lefthook и CI).
- Разные секреты на окружение. Ключ прода никогда не используется локально.

## Исходящие запросы (SSRF)
Вебхуки клиник, загрузка по URL, интеграции: allowlist хостов или резолв + запрет приватных/loopback/link-local/metadata (`169.254.169.254`) диапазонов в `net.Dialer.Control` (защита от DNS rebinding), запрет редиректов на другие хосты, таймауты, лимит размера ответа.

## Входящие вебхуки
HMAC-подпись + проверка timestamp (окно 5 мин) + идемпотентность по event id. `subtle.ConstantTimeCompare`.

## Файлы (анализы, сканы, аудио)
- Приватное S3-совместимое хранилище в РФ, ключ объекта — случайный UUID, без ПДн в имени.
- Загрузка через pre-signed URL с ограничением типа/размера; проверка magic bytes (`http.DetectContentType`) после загрузки, антивирус (ClamAV) в River перед доступностью.
- Скачивание — короткоживущий pre-signed URL после проверки прав + аудит. `Content-Disposition: attachment` для недоверенных типов.
- Шифрование на стороне сервера (SSE) обязательно.

## Ошибки и логи
- Наружу — problem+json с `request_id`, без stack trace, SQL, путей, значений полей.
- `recover` middleware: лог panic с stack, но без тела запроса.
- slog: `ReplaceAttr` + `LogValuer` на типах `Patient`, `PersonName`, `Phone`, `Snils`, ... → `[REDACTED]`. Тело запросов/ответов не логировать. Тест: захват лога и поиск ПДн-маркеров.

## HTTP-заголовки (middleware)
`Strict-Transport-Security: max-age=63072000; includeSubDomains; preload` · `X-Content-Type-Options: nosniff` · `Cache-Control: no-store` для ответов с ПДн · `Referrer-Policy: no-referrer` · CORS: явный allowlist origin, `Allow-Credentials` только для них, никогда `*`.

## Зависимости и CI
- `govulncheck ./...`, `golangci-lint` с `gosec`, `errcheck`, `bodyclose`, `sqlclosecheck`, `noctx`, `contextcheck`.
- Минимальный образ (distroless/static, non-root, read-only FS), сканирование образа (trivy).
- `go.sum` коммитится, новые модули — только из проверенных источников, с обоснованием в PR.

## Анти-паттерны (сразу исправлять)
`math/rand` для секретов · `fmt.Sprintf` в SQL · `InsecureSkipVerify: true` · `http.Get` без таймаута · логирование `req.Body`/`%+v` структур с ПДн · `err.Error()` в ответе клиенту · `*` в CORS с credentials · сравнение токенов через `==` · `os/exec` с пользовательским вводом.
