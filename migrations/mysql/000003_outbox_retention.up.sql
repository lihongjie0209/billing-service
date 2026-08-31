CREATE INDEX billing_outbox_retention_idx ON billing_outbox_events (published_at, id);
