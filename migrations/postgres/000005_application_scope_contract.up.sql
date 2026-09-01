UPDATE subscription_claims c SET application_id=s.application_id FROM subscriptions s WHERE s.id=c.subscription_id AND c.application_id IS NULL;
UPDATE invoices i SET application_id=s.application_id FROM subscriptions s WHERE s.id=i.subscription_id AND i.application_id IS NULL;
UPDATE invoice_generation_keys k SET application_id=i.application_id FROM invoices i WHERE i.id=k.invoice_id AND k.application_id IS NULL;
UPDATE payment_attempts p SET application_id=i.application_id FROM invoices i WHERE i.id=p.invoice_id AND p.application_id IS NULL;
UPDATE refunds r SET tenant_id=p.tenant_id,application_id=p.application_id FROM payment_attempts p WHERE p.id=r.payment_attempt_id AND (r.tenant_id IS NULL OR r.application_id IS NULL);

ALTER TABLE subscriptions ALTER COLUMN application_id SET NOT NULL;
ALTER TABLE subscription_claims ALTER COLUMN application_id SET NOT NULL;
ALTER TABLE invoices ALTER COLUMN application_id SET NOT NULL;
ALTER TABLE invoice_generation_keys ALTER COLUMN application_id SET NOT NULL;
ALTER TABLE payment_attempts ALTER COLUMN application_id SET NOT NULL;
ALTER TABLE refunds ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE refunds ALTER COLUMN application_id SET NOT NULL;

ALTER TABLE subscriptions ADD CONSTRAINT chk_subscriptions_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE subscription_claims ADD CONSTRAINT chk_subscription_claims_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE invoices ADD CONSTRAINT chk_invoices_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE invoice_generation_keys ADD CONSTRAINT chk_invoice_keys_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE payment_attempts ADD CONSTRAINT chk_payments_application_id_nonempty CHECK(application_id <> '');
ALTER TABLE refunds ADD CONSTRAINT chk_refunds_scope_nonempty CHECK(tenant_id <> '' AND application_id <> '');

ALTER TABLE subscription_claims DROP CONSTRAINT subscription_claims_pkey;
ALTER TABLE subscription_claims ADD PRIMARY KEY(tenant_id,application_id);
