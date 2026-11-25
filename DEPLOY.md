# Deploying Vard on a server

One machine running Docker. The stack speaks plain HTTP; TLS and routing are
the infrastructure's, in front of it.

```
infrastructure ─▶ 127.0.0.1:3000  id       ─┐
               └▶ 127.0.0.1:3001  console  ─┤ edge network
                                   api ─────┘ (and out: mail, SSO, logout)
                                    │
                      postgres, redis   data network (internal, no way out)
```

- **Published:** only the two apps, on loopback. Postgres, Redis and the API
  publish nothing; the apps cannot reach Postgres or Redis.
- **The API runs as `vard_app`**, a database role that reads and writes rows
  and nothing more; it migrates as `vard`, the owner, and makes `vard_app`
  (with `POSTGRES_APP_PASSWORD`) on its first start.
- **The API believes only the two apps** about who is calling
  (`VARD_TRUSTED_PROXIES`, their fixed addresses).
- **Containers** run as unprivileged users with no capabilities, read-only
  filesystems (the apps and the API), `no-new-privileges`, memory and process
  limits, and capped logs.

## What you need

- A Linux server with Docker Engine and the Compose plugin, and git.
- Two DNS names pointing at it: one for the sign-in pages (the issuer, such as
  `id.example.com`) and one for the panel (`admin.example.com`).
- A firewall that lets in 22, 80 and 443 only:
  `sudo ufw allow OpenSSH && sudo ufw allow 80,443/tcp && sudo ufw enable`.

## First deployment

```sh
git clone https://github.com/a1i6ek/vard.git && cd vard

# compose.env, with new secrets — back it up: VARD_SECRET_KEY unlocks the
# signing keys, and without it every token and session is lost.
make docker-env public=https://id.example.com admin=https://admin.example.com
$EDITOR compose.env          # the VARD_SMTP_* settings, so mail is sent

make docker-up               # build and start; waits until everything is healthy
```

Route `PUBLIC_URL` to `ID_PUBLISH` and `ADMIN_URL` to `CONSOLE_PUBLISH`,
then open
`https://admin.example.com/new-super-admin` and make the first administrator
straight away: until someone has, whoever opens that page first can.
Every administrator sets up an authenticator app at their first sign-in:
`compose.env` says `VARD_ADMIN_MFA=required`, which a super admin can change
later in Administrators › Two-factor sign-in.

The server refuses to start on plain http addresses other than localhost.
`VARD_ALLOW_INSECURE_HTTP` lets plain http through for a test setup, with a
warning at every start; never set it on a real server. It never lets a weak
`VARD_SECRET_KEY` through: a key of hex digits alone under 64 characters, or
any other key under 43 characters or with fewer than 16 distinct characters,
always stops the server.

## Updating

```sh
git pull
make db-backup               # first, always
make docker-up               # rebuilds what changed, migrates the database on start
```

### A database from before 2026-11-26

The migrations were squashed into one setup on 2026-11-26
(`internal/infrastructure/database/migrations/setup`), as they were once
before. A database an earlier version made names changes in its history this
server does not have, so the server refuses it at start and says so; it is
not stepped forward. Back it up and start on a new, empty database: its rows
are not carried over. A development database is rebuilt with `make db-reset`.
What changed in each release before then is in the git history.

### Upgrading to 1.0.0

1.0.0 refuses to start with a weak `VARD_SECRET_KEY`, whatever else is set: a
key made only of hex digits needs at least 64 characters, and any other key at
least 43 characters and 16 distinct ones. A key made with `openssl rand -base64
32` (what `make docker-env` writes) or `openssl rand -hex 32` passes. The key
cannot be changed: under a new one nothing stored opens, so an installation
with a weak key starts again with a fresh database.

## Rotating the secret key

`VARD_SECRET_KEY` seals every stored secret. To replace it, say after a
suspected leak (ADR 0007):

1. `make db-backup`.
2. In `compose.env`, move the current key into `VARD_OLD_SECRET_KEYS` and set
   a new `VARD_SECRET_KEY` (`openssl rand -base64 32`).
3. `make docker-up`: the server seals new secrets under the new key and still
   opens the old ones.
4. `docker compose exec api /app/vard keys reseal`: it seals every stored secret
   again under the new key, in batches, and says how many in each table. It
   can be stopped and run again.
5. Empty `VARD_OLD_SECRET_KEYS` and `make docker-up` again. The old key opens
   nothing any more.

## Backups

`make db-backup` writes `backups/vard-<time>.dump` and keeps the newest
14 (`KEEP=30 make db-backup` for more). Run it from cron, and copy the dumps
off the machine: they hold every user and every sealed secret.

```
15 3 * * * cd /srv/vard && make db-backup >>/var/log/vard-backup.log 2>&1
```

Restoring is in the comment at the top of `scripts/db-backup.sh`.

## Checking it

```sh
make docker-ps                         # every container healthy
make docker-logs service=api           # the API's log
curl -sI https://id.example.com/.well-known/openid-configuration   # 200, with HSTS
```

## Settings

Everything is in `compose.env`; `compose.env.example` explains each. The ones
a server must not just change: `VARD_SECRET_KEY`, which seals every stored
secret (rotate it as below), and `POSTGRES_PASSWORD` and
`POSTGRES_APP_PASSWORD`, which the database and `vard_app` were made with. `ID_PUBLISH` and
`CONSOLE_PUBLISH` stay on loopback: Docker opens a published port past ufw.
