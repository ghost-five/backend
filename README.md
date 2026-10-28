# Ghost Five: сервер

Партии, аккаунты, друзья, история и рейтинг. Клиент — в соседнем репозитории
[ghost-five/frontend](https://github.com/ghost-five/frontend). Контракт между ними описан в [docs/api](docs/api/README.md) и обязателен
для обеих сторон.

Сервер пока в зачатке: он умеет сказать, что жив. Схема базы, контракт, сборка и
выкладка уже готовы — писать можно сразу маршруты.

## Что уже есть

- `migrations/0001_init.sql` — схема базы. Проверена на Postgres 16: применяется,
  откатывается, ограничения и каскады работают.
- `cmd/api`, `internal/` — каркас сервера на Go. Пока отвечает только «жив»
  (`/healthz`), но этого хватает, чтобы собрать образ, выложить его и убедиться,
  что конвейер работает целиком.
- `Dockerfile`, `deploy/` — боевая сборка и выкладка, см. [deploy/README.md](deploy/README.md).

## Код

```
cmd/api/          точка входа: api (сервер) и api healthcheck (для docker)
internal/config/  настройки из переменных окружения
internal/api/     HTTP: маршруты из docs/api/openapi.yaml
internal/game/    правила партии — порт engine.js (пока только проверка фикстур)
```

Дальше по мере работы появятся `internal/store` (SQL), `internal/service`
(сценарии: аккаунты, лобби, партия, рейтинг) и `internal/ws` (партия по WebSocket).
Слои без магии: `api`/`ws` → `service` → `store`, запросы — обычный SQL.

## Поднять у себя

Нужны Docker с плагином compose и Go 1.27+ (второй — если хотите запускать
сервер без контейнера). Ни баз, ни пакетов ставить не надо.

```bash
git clone https://github.com/ghost-five/backend.git && cd backend

docker compose -f deploy/compose.dev.yml up -d db          # Postgres на localhost:5432
docker compose -f deploy/compose.dev.yml run --rm migrate  # создать таблицы
docker compose -f deploy/compose.dev.yml up -d --build api # сервер на localhost:8080

curl localhost:8080/api/v1/healthz
# {"status":"ok","version":"dev","uptime":3}
```

Убрать за собой вместе с данными: `docker compose -f deploy/compose.dev.yml down -v`.

### Разрабатывать удобнее без контейнера

Контейнер пересобирается на каждую правку, поэтому держите в нём только базу,
а сервер запускайте напрямую:

```bash
docker compose -f deploy/compose.dev.yml up -d db
export DATABASE_URL='postgres://ghost:ghost@localhost:5432/ghost_five?sslmode=disable'
export JWT_SECRET=dev-секрет-не-для-боя
go run ./cmd/api   # Ctrl+C — и снова, после правки
```

### Проверки

То же, что гоняет CI:

```bash
go test ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run
```

Тест `internal/game` читает золотые тесты из соседнего репозитория: положите
`frontend` рядом с `backend` или укажите путь в `GF_FIXTURES`. Без них тест
пропускается локально, а в CI фикстуры забираются всегда.

### Полезное

```bash
# заглянуть в базу
docker compose -f deploy/compose.dev.yml exec db psql -U ghost -d ghost_five

# миграции: применить, откатить последнюю
docker compose -f deploy/compose.dev.yml run --rm migrate
docker compose -f deploy/compose.dev.yml run --rm migrate --no-dump-schema rollback

# журнал сервера в контейнере
docker compose -f deploy/compose.dev.yml logs -f api
```

Переменные окружения сервер читает в [internal/config/config.go](internal/config/config.go).
Пока сервер не ходит в базу, обязательных нет; `DATABASE_URL` станет обязательной
вместе с первым запросом к ней. Без `SMTP_URL` сервер поднимется, но письма слать
откажется.

Проверено: база поднимается, миграции применяются и откатываются, сервер отвечает
и в контейнере, и через `go run`; контейнер становится healthy и по `docker stop`
выходит сам, с кодом 0.

## Где правила партии

Пока — в репозитории [frontend](https://github.com/ghost-five/frontend): там
единственная реализация (`engine.js`) и золотые тесты к ней (`fixtures/`). Фикстуры — это позиция, ходы и состояние, которое должно
получиться; по ним проверяется любая реализация правил.

Когда здесь появится движок на Go, фикстуры переезжают сюда вместе с ним: сервер
станет источником истины, а фронту движок останется нужен только для игры с ботом
и подсветки ходов. До тех пор Go-тесты берут фикстуры из `frontend`
(`fixtures/cases/*.json`) — файлы самодостаточные, им нужен только JSON-разбор.

## Схема базы, вкратце

| Таблица | Что хранит |
|---|---|
| `players` | аккаунт, ник, argon2id-хеш пароля, рейтинг |
| `login_codes` | одноразовые коды подтверждения и сброса — **хешами**, не открыто |
| `sessions` | refresh-токены, чтобы их можно было отозвать |
| `friendships` | заявки и дружба |
| `games` | партия: правила, состояние раунда, счёт, срок текущего хода |
| `pending_moves` | чужой несделанный ход; наружу не отдаётся — в этом вся тайна |
| `history` | сыгранные раунды, своя запись у каждого игрока |

Два отличия от схемы Supabase, о которых стоит помнить:

1. **Нет RLS.** К базе ходит только сервер, одной ролью. Права разруливает код, и
   проверка «ваша ли это партия» — обычное условие в запросе, а не политика.
2. **Нет функций, считающих рейтинг.** Эло считает сервис, а не SQL: правила должны
   быть в одном месте и на одном языке, иначе Go-версия разъедется с базой.

## Миграции

Формат [dbmate](https://github.com/amacneil/dbmate) — метки `-- migrate:up` и
`-- migrate:down` понимает и он, и goose, и самописный runner на десять строк.

Применяет их образ `ghcr.io/amacneil/dbmate:2` — один и тот же локально
(`compose.dev.yml`), в CI и на ВМ (`compose.yml`). Каталог `migrations/` в нём
смонтирован в `/db/migrations`, где dbmate ищет миграции по умолчанию. Если
ставите dbmate себе отдельно, каталог надо указать явно:
`dbmate -d ./migrations --no-dump-schema up`.

Правило: миграции только добавляются. Исправлять уже применённую нельзя — пишется
следующая. Каждая миграция обязана иметь рабочий `down`, иначе откат релиза
превращается в ручную работу на боевой базе.

## Дальше по порядку

1. ~~Каркас сервера на Go: конфиг из переменных окружения, `/healthz`, образ, CI.~~
2. Движок на Go: порт `engine.js`, проходит золотые тесты. Идёт раньше аккаунтов —
   это самая непредсказуемая часть, и от базы она не зависит.
3. Аккаунты: регистрация с кодом, вход, refresh, сброс пароля, письма через SMTP.
4. Лобби: друзья, приглашения, создание партии.
5. Партия: применение хода движком, тайна чужого хода, серверные часы, рассылка
   по WebSocket.
6. История и рейтинг Эло по окончании партии.

Контракт на всё это уже описан: [openapi.yaml](docs/api/openapi.yaml) и
[ws.md](docs/api/ws.md). Общий план переезда —
[docs/architecture.md](https://github.com/ghost-five/frontend/blob/main/docs/architecture.md)
в репозитории frontend.
