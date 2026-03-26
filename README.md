# Payment Gateway

Тестовый проект платёжного шлюза (Payment Gateway), написанный на Go. Проект построен по принципам **Clean Architecture** и использует паттерн **Event Sourcing** (CQRS).

## Технологический стек

- **Язык:** Go 1.25
- **Архитектура:** Clean Architecture, Event Sourcing
- **База данных:** PostgreSQL 16 (pgxpool)
- **Брокер сообщений:** Kafka (segmentio/kafka-go)
- **HTTP:** Chi router
- **Observability:** OpenTelemetry (Traces & Metrics), Jaeger, Prometheus, Grafana
- **Логирование:** slog
- **Инфраструктура:** Docker & Docker Compose

## Архитектура и Компоненты

Система состоит из двух независимых микросервисов:
1. **API Service:** Обрабатывает HTTP-запросы, сохраняет события в Event Store (PostgreSQL) и публикует команды в Kafka (`payments.commands`).
2. **Consumer Service:** Асинхронно читает команды из Kafka, обращается к провайдеру, обновляет статус платежа, пишет новые события и обновляет Read Model (проекцию) для быстрых GET-запросов.

---

## Запуск проекта

```bash
docker compose up -d --build
```

**Доступные сервисы после старта:**
- **API (Приложение):** http://localhost:8080
- **Kafka UI:** http://localhost:8081
- **Jaeger (Трейсы):** http://localhost:16686
- **Prometheus (Сырые метрики):** http://localhost:9090
- **Grafana (Дашборды):** http://localhost:3000 *(login: admin / admin)*

---

## Примеры базовых команд для тестирования (cURL)

### 1. Создать платёж (Create Payment)
Запрос моментально возвращает `201 Created`. В фоне консьюмер начнет его обработку (оно занимает ~3 секунды).

```bash
curl -i -X POST http://localhost:8080/payments \
  -H "Content-Type: application/json" \
  -d '{
    "idempotency_key": "unique-key-12345",
    "amount": 100,
    "currency": "USD"
  }'
```

*(Обратите внимание: чтобы создать **новый** платёж, необходимо менять значение `idempotency_key` (например на `unique-key-12346`)*.

### 2. Получить текущий статус платежа (Get Payment)

```bash
curl -i -X GET http://localhost:8080/payments/{id}
```

### 3. Отменить платёж (Cancel Payment)

```bash
curl -i -X POST http://localhost:8080/payments/{id}/cancel
```

### 4. Посмотреть Prometheus-метрики
Это эндпоинт, который опрашивает сам Prometheus-сервер

```bash
curl -i -X GET http://localhost:8080/metrics
```

---

## Observability (Наблюдаемость)

Проект полностью инструментирован с помощью **OpenTelemetry**:
- **Traces:** Любой HTTP-запрос (к `/payments...`) порождает распределённый трейс. Вызовы в PostgreSQL, публикации в Kafka и обработка внутри Worker'ов прошиты span'ами. Смотреть в **Jaeger**.
- **Metrics:** Подсчет HTTP запросов (`http_requests_total`), задержек HTTP (`http_request_duration_seconds`), и бизнес-статусов платежей (`payment_status_total`). Настроено в **Prometheus** и доступно для аналитики в **Grafana** (дашборды сохраняются в примонтированный volume).
