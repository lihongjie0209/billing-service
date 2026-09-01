# Migrations

Database-specific migrations live under `mysql`, `postgres`, and `kingbase`. Set `migration.path` to the matching directory, for example `migrations/postgres`. Review indexes, collation and online-DDL impact against production data before deployment.

Application scope uses an expand/contract rollout. For an existing database, stop legacy billing writes, apply only `000004`, backfill every `subscriptions.application_id` from the authoritative tenant-application mapping, and verify that no value is null or empty. Deploy the application-aware service and then apply `000005`; it derives invoice, payment, refund, idempotency, and active-subscription claim scope from the subscription chain before enforcing constraints. A new empty environment may apply all migrations in one pass.
