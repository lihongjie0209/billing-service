CREATE OR REPLACE FUNCTION billing_audit_row()
RETURNS trigger LANGUAGE plpgsql AS $audit$
DECLARE actor_id TEXT := NULLIF(current_setting('app.actor_id', true), '');
BEGIN
  IF actor_id IS NULL THEN RAISE EXCEPTION 'app.actor_id must be set for audited writes'; END IF;
  IF TG_OP = 'INSERT' THEN NEW.created_at := statement_timestamp(); NEW.updated_at := NEW.created_at; NEW.created_by := actor_id; NEW.updated_by := actor_id; NEW.version := 1; NEW.deleted_at := NULL; NEW.deleted_by := NULL; RETURN NEW; END IF;
  IF TG_OP = 'UPDATE' THEN NEW.created_at := OLD.created_at; NEW.created_by := OLD.created_by; NEW.updated_at := statement_timestamp(); NEW.updated_by := actor_id; NEW.version := OLD.version + 1; IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN NEW.deleted_at := statement_timestamp(); NEW.deleted_by := actor_id; ELSIF OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL THEN NEW.deleted_by := NULL; ELSE NEW.deleted_at := OLD.deleted_at; NEW.deleted_by := OLD.deleted_by; END IF; RETURN NEW; END IF;
  RAISE EXCEPTION 'physical DELETE is forbidden on audited table %, use deleted_at', TG_TABLE_NAME;
END;
$audit$;

ALTER TABLE plans ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE usage_prices ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE subscriptions ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE subscription_claims ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE invoices ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE invoice_lines ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE invoice_generation_keys ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE payment_attempts ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE payment_provider_events ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE refunds ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;
ALTER TABLE billing_outbox_events ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN deleted_by TEXT;

CREATE TRIGGER plans_audit_row BEFORE INSERT OR UPDATE OR DELETE ON plans FOR EACH ROW EXECUTE FUNCTION billing_audit_row();
CREATE TRIGGER usage_prices_audit_row BEFORE INSERT OR UPDATE OR DELETE ON usage_prices FOR EACH ROW EXECUTE FUNCTION billing_audit_row();
CREATE TRIGGER subscriptions_audit_row BEFORE INSERT OR UPDATE OR DELETE ON subscriptions FOR EACH ROW EXECUTE FUNCTION billing_audit_row();
CREATE TRIGGER subscription_claims_audit_row BEFORE INSERT OR UPDATE OR DELETE ON subscription_claims FOR EACH ROW EXECUTE FUNCTION billing_audit_row();
CREATE TRIGGER invoices_audit_row BEFORE INSERT OR UPDATE OR DELETE ON invoices FOR EACH ROW EXECUTE FUNCTION billing_audit_row();
CREATE TRIGGER invoice_lines_audit_row BEFORE INSERT OR UPDATE OR DELETE ON invoice_lines FOR EACH ROW EXECUTE FUNCTION billing_audit_row();
CREATE TRIGGER invoice_generation_keys_audit_row BEFORE INSERT OR UPDATE OR DELETE ON invoice_generation_keys FOR EACH ROW EXECUTE FUNCTION billing_audit_row();
CREATE TRIGGER payment_attempts_audit_row BEFORE INSERT OR UPDATE OR DELETE ON payment_attempts FOR EACH ROW EXECUTE FUNCTION billing_audit_row();
CREATE TRIGGER payment_provider_events_audit_row BEFORE INSERT OR UPDATE OR DELETE ON payment_provider_events FOR EACH ROW EXECUTE FUNCTION billing_audit_row();
CREATE TRIGGER refunds_audit_row BEFORE INSERT OR UPDATE OR DELETE ON refunds FOR EACH ROW EXECUTE FUNCTION billing_audit_row();

COMMENT ON TABLE billing_outbox_events IS 'High-volume bounded-retention exception: audit columns are populated by the owning transaction; published rows are physically purged in bounded batches.';
