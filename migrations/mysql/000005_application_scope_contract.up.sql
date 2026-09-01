UPDATE subscription_claims c JOIN subscriptions s ON s.id=c.subscription_id SET c.application_id=s.application_id WHERE c.application_id IS NULL;
UPDATE invoices i JOIN subscriptions s ON s.id=i.subscription_id SET i.application_id=s.application_id WHERE i.application_id IS NULL;
UPDATE invoice_generation_keys k JOIN invoices i ON i.id=k.invoice_id SET k.application_id=i.application_id WHERE k.application_id IS NULL;
UPDATE payment_attempts p JOIN invoices i ON i.id=p.invoice_id SET p.application_id=i.application_id WHERE p.application_id IS NULL;
UPDATE refunds r JOIN payment_attempts p ON p.id=r.payment_attempt_id SET r.tenant_id=p.tenant_id,r.application_id=p.application_id WHERE r.tenant_id IS NULL OR r.application_id IS NULL;

ALTER TABLE subscriptions MODIFY COLUMN application_id VARCHAR(191) NOT NULL, ADD CONSTRAINT chk_subscriptions_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE subscription_claims DROP PRIMARY KEY, MODIFY COLUMN application_id VARCHAR(191) NOT NULL, ADD PRIMARY KEY(tenant_id,application_id), ADD CONSTRAINT chk_subscription_claims_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE invoices MODIFY COLUMN application_id VARCHAR(191) NOT NULL, ADD CONSTRAINT chk_invoices_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE invoice_generation_keys MODIFY COLUMN application_id VARCHAR(191) NOT NULL, ADD CONSTRAINT chk_invoice_keys_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE payment_attempts MODIFY COLUMN application_id VARCHAR(191) NOT NULL, ADD CONSTRAINT chk_payments_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE refunds MODIFY COLUMN tenant_id VARCHAR(191) NOT NULL, MODIFY COLUMN application_id VARCHAR(191) NOT NULL, ADD CONSTRAINT chk_refunds_scope_nonempty CHECK(tenant_id <> '' AND application_id <> '');
