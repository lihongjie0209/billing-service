ALTER TABLE refunds DROP CHECK chk_refunds_scope_nonempty, MODIFY COLUMN tenant_id VARCHAR(191) NULL, MODIFY COLUMN application_id VARCHAR(191) NULL;
ALTER TABLE payment_attempts DROP CHECK chk_payments_application_id_nonempty, MODIFY COLUMN application_id VARCHAR(191) NULL;
ALTER TABLE invoice_generation_keys DROP CHECK chk_invoice_keys_application_id_nonempty, MODIFY COLUMN application_id VARCHAR(191) NULL;
ALTER TABLE invoices DROP CHECK chk_invoices_application_id_nonempty, MODIFY COLUMN application_id VARCHAR(191) NULL;
ALTER TABLE subscription_claims DROP CHECK chk_subscription_claims_application_id_nonempty, DROP PRIMARY KEY, MODIFY COLUMN application_id VARCHAR(191) NULL, ADD PRIMARY KEY(tenant_id);
ALTER TABLE subscriptions DROP CHECK chk_subscriptions_application_id_nonempty, MODIFY COLUMN application_id VARCHAR(191) NULL;
