# Ghost Five: сервер

Партии, аккаунты, друзья, история и рейтинг. Клиент — в соседнем репозитории
[ghost-five/frontend](https://github.com/ghost-five/frontend). Контракт между ними описан в [docs/api](docs/api/README.md) и обязателен
для обеих сторон.

Сервер пока в зачатке: он умеет сказать, что жив. Схема базы, контракт, сборка и
выкладка уже готовы — писать можно сразу маршруты.

## Что уже есть

- `migrations/0001_init.sql` — схема базы. Проверена на Postgres 16: применяется,
  откатывается, ограничения и каскады работают.
- `src/` — каркас сервера. Пока отвечает только «жив» (`/healthz`), но этого хватает,
  чтобы собрать образ, выложить его и убедиться, что конвейер работает целиком.
- `Dockerfile`, `deploy/` — боевая сборка и выкладка, см. [deploy/README.md](deploy/README.md).

## Поднять у себя

Нужны только Docker с плагином compose и Node 22+ (второй — если хотите запускать
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
а сервер запускайте напрямую — он перечитывает файлы сам:

```bash
docker compose -f deploy/compose.dev.yml up -d db
export DATABASE_URL='postgres://ghost:ghost@localhost:5432/ghost_five?sslmode=disable'
export JWT_SECRET=dev-секрет-не-для-боя
npm run dev        # node --watch src/server.js
```

### Полезное

```bash
# заглянуть в базу
docker compose -f deploy/compose.dev.yml exec db psql -U ghost -d ghost_five

# миграции: применить, откатить последнюю
npx dbmate -d ./migrations --no-dump-schema up
npx dbmate -d ./migrations --no-dump-schema rollback

# журнал сервера в контейнере
docker compose -f deploy/compose.dev.yml logs -f api
```

Переменные окружения сервер читает в [src/config.js](src/config.js); обязательна
только `DATABASE_URL`. Без `SMTP_URL` он поднимется, но письма слать откажется.

Проверено на чистом клоне: база поднимается, миграции применяются, сервер отвечает
и в контейнере, и напрямую через `node`.

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

```
export DATABASE_URL='postgres://ghost:пароль@localhost:5432/ghost_five?sslmode=disable'
npm run migrate    # применить
npm run rollback   # откатить последнюю
```

Внутри — `dbmate -d ./migrations`: каталог указываем явно, потому что по умолчанию
dbmate ищет миграции в `db/migrations`. В контейнерах они и смонтированы туда, так
что там флаг не нужен.

Правило: миграции только добавляются. Исправлять уже применённую нельзя — пишется
следующая. Каждая миграция обязана иметь рабочий `down`, иначе откат релиза
превращается в ручную работу на боевой базе.

## Дальше по порядку

1. Каркас сервера: `transport` (HTTP + WebSocket) → `service` → `store`, конфиг из
   переменных окружения, `/healthz`.
2. Аккаунты: регистрация с кодом, вход, refresh, сброс пароля, письма через SMTP.
3. Лобби: друзья, приглашения, создание партии.
4. Партия: применение хода движком, тайна чужого хода, серверные часы, рассылка
   по WebSocket.
5. История и рейтинг Эло по окончании партии.

Контракт на всё это уже описан: [openapi.yaml](docs/api/openapi.yaml) и
[ws.md](docs/api/ws.md). Общий план переезда —
[docs/architecture.md](https://github.com/ghost-five/frontend/blob/main/docs/architecture.md)
в репозитории frontend.
