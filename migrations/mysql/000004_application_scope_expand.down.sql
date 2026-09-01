ALTER TABLE refunds DROP INDEX idx_refunds_application_created, DROP COLUMN application_id, DROP COLUMN tenant_id;
ALTER TABLE payment_attempts DROP INDEX idx_payment_attempts_application_created, DROP COLUMN application_id;
ALTER TABLE invoice_generation_keys DROP COLUMN application_id;
ALTER TABLE invoices DROP INDEX idx_invoices_application_status_created, DROP COLUMN application_id, ADD INDEX idx_invoices_tenant_status_created(tenant_id,status,created_at);
ALTER TABLE subscription_claims DROP COLUMN application_id;
ALTER TABLE subscriptions DROP INDEX idx_subscriptions_application_status, DROP COLUMN application_id, ADD INDEX idx_subscriptions_tenant_status(tenant_id,status,updated_at);
