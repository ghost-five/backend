-- Ghost Five: начальная схема своего бэкенда.
-- Формат dbmate (он же понятен goose и любому runner'у с такими же метками).
-- Главное отличие от схемы Supabase: к базе ходит ТОЛЬКО сервер, одной ролью.
-- Поэтому здесь нет RLS и нет функций, считающих рейтинг: правила и рейтинг —
-- в коде сервера, а база хранит данные и следит за их целостностью.

-- migrate:up

create extension if not exists pgcrypto;   -- gen_random_uuid()
create extension if not exists citext;     -- почта без учёта регистра

-- ---------- Игроки ----------

create table players (
  id                 uuid primary key default gen_random_uuid(),
  email              citext not null unique,
  email_confirmed_at timestamptz,
  nickname           text not null,
  -- argon2id, в виде строки PHC ($argon2id$v=19$m=...$...$...)
  password_hash      text not null,
  rating             int not null default 1000,
  wins               int not null default 0,
  losses             int not null default 0,
  created_at         timestamptz not null default now(),
  updated_at         timestamptz not null default now(),
  constraint players_nickname_format check (nickname ~ '^[A-Za-zА-Яа-яЁё0-9_.-]{3,20}$'),
  constraint players_rating_sane check (rating between 100 and 4000)
);

-- Ник уникален без учёта регистра: «Один» и «один» — один и тот же игрок.
create unique index players_nickname_key on players (lower(nickname));
create index players_rating_idx on players (rating desc);

-- ---------- Одноразовые коды: подтверждение почты и сброс пароля ----------
-- Сам код не храним — только его хеш, как пароль. Проверка: найти живой код
-- игрока нужного вида, сверить хеш, увеличить attempts, при успехе пометить used_at.

create table login_codes (
  id         uuid primary key default gen_random_uuid(),
  player_id  uuid not null references players (id) on delete cascade,
  kind       text not null check (kind in ('confirm', 'reset')),
  code_hash  text not null,
  attempts   int not null default 0,
  expires_at timestamptz not null,
  used_at    timestamptz,
  created_at timestamptz not null default now()
);

create index login_codes_live_idx on login_codes (player_id, kind) where used_at is null;

-- ---------- Сессии: refresh-токены ----------
-- Access-токен короткий и в базе не лежит. Refresh живёт здесь, чтобы его можно
-- было отозвать: при выходе, при смене пароля и при подозрении на кражу.

create table sessions (
  id         uuid primary key default gen_random_uuid(),
  player_id  uuid not null references players (id) on delete cascade,
  token_hash text not null unique,
  user_agent text,
  ip         inet,
  expires_at timestamptz not null,
  revoked_at timestamptz,
  created_at timestamptz not null default now()
);

create index sessions_player_idx on sessions (player_id) where revoked_at is null;

-- ---------- Друзья ----------

create table friendships (
  requester  uuid not null references players (id) on delete cascade,
  addressee  uuid not null references players (id) on delete cascade,
  status     text not null default 'pending' check (status in ('pending', 'accepted')),
  created_at timestamptz not null default now(),
  primary key (requester, addressee),
  constraint friendship_not_self check (requester <> addressee)
);

create index friendships_addressee_idx on friendships (addressee);

-- ---------- Партии ----------
-- state — состояние раунда по docs/api/game-state.schema.json.
-- Его пишет только сервер, применив правила к ходам: клиент сюда не дотягивается.

create table games (
  code       text primary key,
  p1         uuid not null references players (id) on delete cascade,
  p2         uuid references players (id) on delete set null,
  invitee    uuid references players (id) on delete set null,
  status     text not null default 'waiting' check (status in ('waiting', 'playing', 'over', 'declined')),
  cfg        jsonb not null,
  state      jsonb not null,
  score      jsonb not null default '{"1": 0, "2": 0, "d": 0}'::jsonb,
  -- Партия идёт до двух побед; rated ставится, когда рейтинг за неё уже начислен.
  rated      boolean not null default false,
  -- До какого момента идёт текущий ход. Часы считает сервер, клиент только рисует.
  deadline   timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint games_code_format check (code ~ '^[A-Z0-9]{4,8}$'),
  constraint games_two_players check (p2 is null or p2 <> p1)
);

create index games_p1_idx on games (p1);
create index games_p2_idx on games (p2);
create index games_invitee_idx on games (invitee) where invitee is not null;
create index games_updated_idx on games (updated_at desc);
-- Для часов: сервер забирает отсюда партии, где время на ход истекло.
create index games_deadline_idx on games (deadline) where status = 'playing';

-- ---------- Ход, ожидающий соперника ----------
-- При тайном ходе оба выбирают одновременно. Чужой ход лежит здесь и наружу
-- не отдаётся — в этом вся тайна. Соперник узнаёт только, что ход сделан.
-- Пара (game_code, seat) одна: новый ход игрока заменяет прежний, пока не сходил второй.

create table pending_moves (
  game_code  text not null references games (code) on delete cascade,
  seat       smallint not null check (seat in (1, 2)),
  game_no    int not null,
  round      int not null,
  move       jsonb not null,
  created_at timestamptz not null default now(),
  primary key (game_code, seat)
);

-- ---------- История сыгранных раундов ----------
-- Своя у каждого игрока: сетевой раунд кладут себе оба, по записи каждому.

create table history (
  id        uuid primary key default gen_random_uuid(),
  owner     uuid not null references players (id) on delete cascade,
  played_at timestamptz not null default now(),
  mode      text not null default 'online' check (mode in ('online', 'bot', 'friend')),
  code      text,
  game_no   int not null default 1,
  cfg       jsonb not null,
  frames    jsonb not null,
  log       jsonb not null,
  winner    int not null default 0 check (winner between 0 and 2),
  reason    text not null default '',
  score     jsonb,
  names     jsonb
);

create index history_owner_idx on history (owner, played_at desc);
-- Один и тот же сетевой раунд не должен лечь в историю дважды.
create unique index history_round_key on history (owner, code, game_no) where code is not null;

-- ---------- updated_at ----------
-- Единственный триггер в схеме: проставляет время правки, чтобы об этом не помнил
-- каждый запрос. Всё остальное делает код сервера явно.

create or replace function touch_updated_at() returns trigger language plpgsql as $$
begin
  new.updated_at := now();
  return new;
end
$$;

create trigger players_touch before update on players
  for each row execute function touch_updated_at();
create trigger games_touch before update on games
  for each row execute function touch_updated_at();

-- migrate:down

drop trigger if exists games_touch on games;
drop trigger if exists players_touch on players;
drop function if exists touch_updated_at();
drop table if exists history;
drop table if exists pending_moves;
drop table if exists games;
drop table if exists friendships;
drop table if exists sessions;
drop table if exists login_codes;
drop table if exists players;
