-- F016 US5 (data requests, ADR-0070): createDataRequest records who asked and the case reference next to
-- the anonymize request (DataRequestCreate.requester_kind, reference). Hand-written table: expand only.
ALTER TABLE platform.deletion_requests
  ADD COLUMN requester_kind varchar(32) CHECK (requester_kind IN ('school', 'parent_via_school', 'student', 'authority')),
  ADD COLUMN reference varchar(128);
-- One open request per subject already exists (deletion_requests_open_uq); list by tenant and time.
CREATE INDEX deletion_requests_tenant_idx ON platform.deletion_requests (tenant_id, created_at DESC);
