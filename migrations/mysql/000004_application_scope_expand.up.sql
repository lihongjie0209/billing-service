ALTER TABLE subscriptions ADD COLUMN application_id VARCHAR(191) NULL AFTER tenant_id, DROP INDEX idx_subscriptions_tenant_status, ADD INDEX idx_subscriptions_application_status(tenant_id,application_id,status,updated_at);
ALTER TABLE subscription_claims ADD COLUMN application_id VARCHAR(191) NULL AFTER tenant_id;
ALTER TABLE invoices ADD COLUMN application_id VARCHAR(191) NULL AFTER tenant_id, DROP INDEX idx_invoices_tenant_status_created, ADD INDEX idx_invoices_application_status_created(tenant_id,application_id,status,created_at);
ALTER TABLE invoice_generation_keys ADD COLUMN application_id VARCHAR(191) NULL AFTER tenant_id;
ALTER TABLE payment_attempts ADD COLUMN application_id VARCHAR(191) NULL AFTER tenant_id, ADD INDEX idx_payment_attempts_application_created(tenant_id,application_id,created_at);
ALTER TABLE refunds ADD COLUMN tenant_id VARCHAR(191) NULL AFTER invoice_id, ADD COLUMN application_id VARCHAR(191) NULL AFTER tenant_id, ADD INDEX idx_refunds_application_created(tenant_id,application_id,created_at);
