# 0008 Durable back-channel logout, and none on expiry

- **Status:** Accepted (2026-11-20)
- **Date:** 2026-11-20

## Context

An application with a `backchannel_logout_uri` is told, server to server,
when a session it signed someone in through ends (OpenID Connect
Back-Channel Logout 1.0). Until now the notice was posted from memory after
the request that ended the session, tried three times over ten seconds.
A restart, a deploy or an application down for a minute lost it, and the
application kept a session the user had ended.

Sessions also end by running out: nobody ends them, their `expires_at`
passes. Whether that should notify was an open question.

## Decision

**An outbox, written in the transaction that ends the session.**

- **Queue:** `revokeUserSessionsIn`, which every way of ending a session goes
  through, writes a `logout_deliveries` row for each application to tell, in
  the same transaction. A session that ended is owed its notices, and one
  that didn't end owes none.
- **Send:** a worker on every instance (`oidc.KeepTelling`) claims due rows
  with `FOR UPDATE SKIP LOCKED`. It puts each off by a one-minute lease, so
  another instance skips it and a sender that died mid-try is replaced. The
  commit wakes the worker at once; otherwise it looks every two seconds.
- **Outcome:**
  - a 200 or 204 is **delivered**;
  - a 4xx is **failed** at once, since another try would get the same answer;
  - anything else is tried again after 5 s, doubling, at most an hour
    apart, for up to 24 hours (`model.LogoutRetryAfter`, `LogoutRetryFor`),
    and then **failed**.

  A failed notice is written to the activity log
  (`application.logout_undelivered`), in the same transaction that marks it.

- **Kept:** each notice is kept a week past its tries, and the panel shows
  an application's (Applications → the application → Logouts).
- **Token:** the logout token is signed afresh for each try.

**A session that expires notifies nobody.** Only a session someone ends
does: the user, an application's logout, an administrator, a password
change or reset, turning the user off, deleting them. Ending a session that
has already expired, with all the user's others, tells no application
either.

## Alternatives rejected

- **Notify on expiry.** The sweep would have to queue notices for every
  session it removes. Expiry isn't a logout in OpenID Connect's sense, and
  an application already learns of it when a refresh fails or its own
  session times out. Sessions that expired long ago would all notify at
  once on the first sweep after an upgrade.
- **Retry in memory, longer.** Still lost to a restart, and to an instance
  stopped while it waits.
- **A queue outside Postgres (Redis streams, a broker).** Redis is
  optional and never the only copy of anything; a broker is another service
  to run. The outbox commits with the session's end, which nothing outside
  the transaction can.

## Consequences

- **Durable:** a notice survives a restart and an application's outage of
  up to a day, on any number of instances.
- **Once, mostly:** an application may be told twice, if a sender dies
  after the post and before recording it, and the lease runs out. A logout
  token is idempotent for the application: the session is already gone.
- **New table and worker:** `logout_deliveries` (migration 00024), swept
  with everything that expires, and one worker per instance.
- **Expiry:** an application that must know of an expired session finds out
  from its own session's lifetime or from a refresh that fails.
