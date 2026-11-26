# Выбор и настройка мониторинга в системе

## 1. Мотивация

Яндекс Метрика видит только фронтенд `Internet Shop` (B2C) и не покрывает B2B API (`API user` -> `MES API`), `RabbitMQ`, `CRM API` и БД.

- **Меньше потерянных контрактов и жалоб.** Зависшие заказы и сбои API фиксируются по алертам до нарушения срока в три недели.
- **Измеримая база для улучшений.** Видны реальное время расчета 3D-моделей, нагрузка B2B, причина тормозов первой страницы `MES` и эффект от доработок.
- **Прогноз ресурсов.** На единственных EC2 и БД заранее видно приближение к лимитам CPU, RAM, соединений и диска при росте +100 заказов/мес.

---

## 2. Выбор подхода к мониторингу

| Слой | Компоненты | Подход | Обоснование |
| ---- | ---------- | ------ | ----------- |
| Прикладные API | `Shop API`, `CRM API`, `MES API` | **RED** (Rate, Errors, Duration) | Показывает качество обслуживания B2C, операторов, продавцов и B2B-партнеров |
| Инфраструктура и хранилища | EC2, `Shop DB`, `MES DB`, `RabbitMQ`, `S3` | **USE** (Utilization, Saturation, Errors) | Показывает исчерпание ресурсов на одиночных узлах и рост очередей |

---

## 3. Метрики для отслеживания

### API (RED)

| Метрика | Компонент | Зачем нужна | Ярлыки |
| ------- | --------- | ----------- | ------ |
| `Number of requests (RPS)` | `Shop API`, `CRM API`, `MES API` | Объем трафика и рост нагрузки по каналам | Да: `service`, `endpoint`, `method` (в `MES API` также `source`: `ui`, `b2b`) |
| `Number of requests (RPS) per user` | `MES API` | Поиск B2B-партнера с пиковым трафиком и частых обновлений дашборда | Да: `client_id` (B2B) или `role` (`operator`, `b2b`); для `Shop API` не берется из-за высокой кардинальности |
| `Response time (latency)` | `Shop API`, `CRM API`, `MES API` | Контроль задержек первой страницы `MES` и таймаутов B2B (p50/p95/p99) | Да: `service`, `endpoint`, `method` |
| `Number of HTTP 200`, `Number of HTTP 500` | `Shop API`, `CRM API`, `MES API` | Расчет доли ошибок (Error Rate) и фиксация серверных падений | Да: `service`, `endpoint`, `status_code` |
| `Number of HTTP 4xx` (доп.) | `Shop API`, `MES API` | Ошибки валидации и интеграции на стороне клиентов и B2B-партнеров | Да: `service`, `endpoint`, `status_code` |
| `Kb transferred (received)` | `Shop API`, `MES API` | Контроль объема входящих 3D-моделей | Да: `service`, `endpoint` |

### Инфраструктура, БД и хранилище (USE)

| Метрика | Компонент | Зачем нужна | Ярлыки |
| ------- | --------- | ----------- | ------ |
| `CPU %`, `Memory Utilisation` | `Shop API`, `CRM API`, `MES API` | Загрузка одиночных EC2 и утечки памяти (особенно при расчете 3D в `MES`) | Да: `service`, `instance` |
| `Memory Utilisation`, `CPU %` и `Disk I/O` (доп.) | `Shop DB`, `MES DB` | Нагрузка PostgreSQL и диагностика тяжелых выборок первой страницы `MES` | Да: `db_instance` |
| `Number of connections` | `Shop DB`, `MES DB` | Риск исчерпания пула соединений (`Shop DB` делят `Shop API` и `CRM API`) | Да: `db_instance` (в `Shop DB` также `client_app`) |
| `Size of shop db instance`, `Size of MES db instance` | `Shop DB`, `MES DB` | Прогноз роста БД и защита от переполнения диска | Да: `db_instance` |
| `Size of S3 storage` | `3d files storage` (`S3`) | Контроль роста объема 3D-моделей | Да: `bucket` |

### Очередь и бизнес-метрики

| Метрика | Компонент | Зачем нужна | Ярлыки |
| ------- | --------- | ----------- | ------ |
| `Number of dead-letter-exchange letters in RabbitMQ` | `RabbitMQ` | Необработанные сообщения и риск потери заказов (включая B2B) | Да: `queue`, `exchange` |
| `Number of messages in flight in RabbitMQ` | `RabbitMQ` | Насыщение потребителей и зависание обработки | Да: `queue` |
| Время расчета 3D-модели (доп.) | `MES API` | Прямой замер главного узкого места (2-30 мин, p50/p95/p99) | Да: `channel` (`b2c`, `b2b`) |
| Количество и возраст заказов по статусам (доп.) | `Shop`, `CRM`, `MES` | Поиск зависших заказов до нарушения срока в три недели | Да: `status`, `channel` (`b2c`, `b2b`) |

---

## 4. План действий

1. Развернуть на отдельном узле `Prometheus` (time-series БД), `Grafana` и `Alertmanager`, настроить сбор по окружениям `dev`, `release`, `prod` с ограничением кардинальности ярлыков.
2. Подключить экспортеры инфраструктуры: `node_exporter` на EC2, метрики Managed PostgreSQL и `S3` (Yandex Monitoring / `postgres_exporter`), плагин `rabbitmq_prometheus` в `RabbitMQ`.
3. Инструментировать приложения: `Micrometer` + `Spring Boot Actuator` в `Shop API` и `CRM API`, `prometheus-net` (или OpenTelemetry) в `MES API`, включая бизнес-метрики времени расчета и статусов заказов.
4. Собрать базовые дашборды в `Grafana`: поток заказов и SLA, RED по API, USE по EC2, БД, `RabbitMQ` и `S3`.
5. Настроить алерты в `Alertmanager` (канал команды) на `HTTP 5xx`, рост `DLQ`, перегрузку CPU/соединений БД и зависшие заказы.
6. Проверить сбор метрик и алерты на `dev`/`release`, затем выкатить в `prod`.
