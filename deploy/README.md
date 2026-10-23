# Своя ВМ: развернуть и катить обновления

Пошагово. Всё, что ниже, делается один раз, дальше выкладка идёт сама по `git push`.

## 1. Машина

Берём ВМ 2 vCPU / 4 ГБ / 40 ГБ SSD, Ubuntu 24.04 (Selectel, Timeweb, Yandex Cloud —
любой). Этого хватит с большим запасом: игра лёгкая, тяжёлое в ней — только база.

```bash
# от root, один раз
adduser --disabled-password --gecos "" deploy
mkdir -p /home/deploy/.ssh && cp ~/.ssh/authorized_keys /home/deploy/.ssh/
chown -R deploy:deploy /home/deploy/.ssh && chmod 700 /home/deploy/.ssh

# docker
curl -fsSL https://get.docker.com | sh
usermod -aG docker deploy

# закрываем всё лишнее
ufw allow 22/tcp && ufw allow 80/tcp && ufw allow 443/tcp && ufw --force enable
apt-get install -y fail2ban

# вход по паролю выключаем совсем
sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
systemctl restart ssh
```

## 2. Домен

Две записи у регистратора, обе на IP машины:

```
@     A    203.0.113.10
www   A    203.0.113.10
```

Больше ничего делать не надо: сертификат Caddy получит сам при первом запуске.
Пока DNS не разошёлся (до часа), Caddy будет повторять попытки — это нормально.

## 3. Файлы на машине

```bash
sudo mkdir -p /opt/ghost-five/backups && sudo chown -R deploy:deploy /opt/ghost-five
# с локальной машины:
scp backend/deploy/compose.yml backend/deploy/Caddyfile deploy@ВАШ_IP:/opt/ghost-five/
scp backend/deploy/.env.example deploy@ВАШ_IP:/opt/ghost-five/.env
ssh deploy@ВАШ_IP 'chmod 600 /opt/ghost-five/.env'
```

Заполнить `.env`: домен, пароль базы, секрет токенов, SMTP. Пароли генерируем,
а не придумываем: `openssl rand -base64 32`.

Миграции лежат рядом: `scp -r backend/migrations deploy@ВАШ_IP:/opt/ghost-five/`.

## 4. Первый запуск

```bash
cd /opt/ghost-five
echo ТОКЕН | docker login ghcr.io -u ВАШ_ЛОГИН --password-stdin   # или сделать пакеты публичными
docker compose up -d db
docker compose run --rm migrate      # создаёт таблицы
docker compose up -d
docker compose ps                    # все healthy?
curl -sS https://ВАШ_ДОМЕН/api/v1/healthz
```

## 5. Секреты в GitHub

В настройках каждого репозитория (`Settings → Secrets → Actions`), окружение
`production`:

| Секрет | Что это |
|---|---|
| `DEPLOY_HOST` | IP машины |
| `DEPLOY_USER` | `deploy` |
| `DEPLOY_KEY` | приватный ssh-ключ пользователя `deploy` (отдельный, не ваш личный) |

Для публикации образов хватает встроенного `GITHUB_TOKEN` — ничего заводить не надо.

Полезно включить у окружения `production` ручное подтверждение: тогда выкладка
ждёт вашего нажатия, а не уезжает на боевую сразу после слияния.

## 6. Обычный день

```
push в main → ci прогоняет тесты → собирается образ → ssh на ВМ:
  .env обновляется новым тегом → pull → миграции → up -d → ждём healthy
```

**Откат** — две команды на машине:

```bash
sed -i 's|^API_IMAGE=.*|API_IMAGE=ghcr.io/ghost-five/backend:ПРЕДЫДУЩИЙ_SHA|' .env
docker compose up -d api
```

Поэтому тег образа — хеш коммита, а не `latest`: на `latest` непонятно, что именно
сейчас работает, и откатываться некуда.

## 7. Бэкапы

Снимок базы раз в сутки, хранить две недели, копию — наружу (объектное хранилище).
Строка в `crontab -e` пользователя `deploy`:

```cron
15 4 * * * cd /opt/ghost-five && docker compose exec -T db pg_dump -U ghost ghost_five | gzip > backups/$(date +\%F).sql.gz && find backups -name '*.sql.gz' -mtime +14 -delete
```

Копию наружу — `rclone copy backups remote:ghost-five-backups` следующей строкой.

**Бэкап, который ни разу не разворачивали, бэкапом не считается.** Раз в месяц:

```bash
gunzip -c backups/2026-11-01.sql.gz | docker compose exec -T db psql -U ghost -d postgres \
  -c 'create database restore_test' -f -
```

## 8. Предпросмотр

Тот же compose с другим `.env`: домен `stage.ваш-домен`, своя база, свои образы.
Каталог `/opt/ghost-five-stage`. Полезно гонять туда каждый коммит из main, а на
боевую выкладывать по кнопке.

## 9. Что смотреть, когда что-то не так

```bash
docker compose ps                 # кто не healthy
docker compose logs -f --tail=100 api
docker compose logs -f caddy      # сертификаты и маршруты
docker stats                      # память и процессор
```

Журналы растут: ограничьте их в `/etc/docker/daemon.json`
(`{"log-driver":"json-file","log-opts":{"max-size":"10m","max-file":"3"}}`)
и перезапустите docker, иначе однажды кончится диск.
