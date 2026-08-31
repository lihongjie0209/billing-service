CREATE INDEX billing_outbox_retention_idx ON billing_outbox_events (published_at, id) WHERE published_at IS NOT NULL;
