ALTER TABLE subscriptions ADD COLUMN application_id TEXT NULL;
ALTER TABLE subscription_claims ADD COLUMN application_id TEXT NULL;
ALTER TABLE invoices ADD COLUMN application_id TEXT NULL;
ALTER TABLE invoice_generation_keys ADD COLUMN application_id TEXT NULL;
ALTER TABLE payment_attempts ADD COLUMN application_id TEXT NULL;
ALTER TABLE refunds ADD COLUMN tenant_id TEXT NULL;
ALTER TABLE refunds ADD COLUMN application_id TEXT NULL;

DROP INDEX idx_subscriptions_tenant_status;
CREATE INDEX idx_subscriptions_application_status ON subscriptions(tenant_id,application_id,status,updated_at DESC);
DROP INDEX idx_invoices_tenant_status_created;
CREATE INDEX idx_invoices_application_status_created ON invoices(tenant_id,application_id,status,created_at DESC);
CREATE INDEX idx_payment_attempts_application_created ON payment_attempts(tenant_id,application_id,created_at DESC);
CREATE INDEX idx_refunds_application_created ON refunds(tenant_id,application_id,created_at DESC);
