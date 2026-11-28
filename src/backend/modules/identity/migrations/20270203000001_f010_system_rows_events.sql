-- F010 (Clarifications 2026-11-28, [Confirmed — owner "chốt hết"]) — announce the rows that identity migrations created
-- without a use case, so the projections of the other modules receive them (DATABASE_DESIGN §2.17, EVENT_DESIGN §4.1).
-- The common tenant and its virtual school (20261119000002_f010_seed) and the province catalog (same migration, ADR-0060)
-- never went through CreateTenant / a school or catalog use case, so identity never wrote tenant.created, school.updated
-- or province.updated for them: ref.tenants / ref.schools of learning, game, competition and chat had no row for
-- `common` (assignment to common failed on the FK, quest assignments skipped) and ref.provinces of learning was empty.
--
-- Expand only (R12): one INSERT … SELECT of the missing events into infra.outbox_events, nothing updated or deleted, no
-- temporary object. The relay publishes them like any other event; every consumer is idempotent (infra.processed_events)
-- and applies a newer source_version only, so a re-delivery is harmless. Guarded by NOT EXISTS (an event of the same
-- type for the same subject already in the outbox = its consumers already got it): a second apply inserts nothing.
-- Payloads mirror the use cases (tenantPayload, schoolPayload, provinceEvent of module identity) and are read from the
-- current rows; envelope of pkg/outbox (actor system, producer identity/migration). Event ids are UUIDv7 of the migration
-- time with a counter in rand_a (tenant 0, school 1, provinces 2…) so the relay (ORDER BY event_id) publishes the tenant
-- before its school (FK ref.schools → ref.tenants). Applied as identity_migrator (owner of infra.outbox_events);
-- org.schools has FORCE RLS: app.tenant_id is the common tenant for this transaction while it is read.

SELECT set_config('app.tenant_id', coalesce((SELECT tenant_id::text FROM platform.tenants
  WHERE tenant_type = 'common' AND deleted_at IS NULL), ''), true);

INSERT INTO infra.outbox_events (event_id, event_type, subject, tenant_id, partition_key, envelope, created_at)
SELECT e.id, r.typ, 'ew.evt.identity.' || r.typ, r.tenant_id, r.subject_id,
  jsonb_build_object('event_id', e.id, 'event_type', r.typ, 'schema_version', 1, 'occurred_at', c.ts, 'recorded_at', c.ts,
    'producer', 'identity/migration', 'tenant_id', r.tenant_id, 'actor', jsonb_build_object('type', 'system', 'id', NULL),
    'subject', jsonb_build_object('type', r.subject_type, 'id', r.subject_id),
    'idempotency_key', r.typ || ':' || r.subject_id || ':' || e.id, 'trace_id', NULL, 'payload', r.payload),
  now()
FROM (SELECT lpad(to_hex((extract(epoch FROM now()) * 1000)::bigint), 12, '0') AS ms,
        to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS ts) c
CROSS JOIN (
  -- tenant.created of the common tenant (the only tenant_type = 'common').
  SELECT 0::bigint AS seq, 'tenant.created' AS typ, t.tenant_id, 'tenant' AS subject_type, t.tenant_id::text AS subject_id,
    jsonb_build_object('tenant_id', t.tenant_id, 'code', t.code, 'name', t.name, 'tenant_type', t.tenant_type,
      'status', t.status, 'keycloak_org_alias', t.keycloak_org_alias, 'is_qa', t.is_qa, 'changed_fields', '[]'::jsonb,
      'source_version', t.version) AS payload
  FROM platform.tenants t
  WHERE t.tenant_type = 'common' AND t.deleted_at IS NULL
  UNION ALL
  -- school.updated (creation of a school: changed_fields = []) of the virtual school of the common tenant.
  SELECT 1::bigint, 'school.updated', s.tenant_id, 'school', s.school_id::text,
    jsonb_build_object('school_id', s.school_id, 'tenant_id', s.tenant_id, 'status', s.status,
      'name', coalesce(s.name_i18n->>'vi', s.name),
      'name_i18n', jsonb_build_object('vi', coalesce(s.name_i18n->>'vi', s.name), 'en', coalesce(s.name_i18n->>'en', s.name)),
      'province_code', p.code, 'school_level', s.school_level, 'is_virtual', s.is_virtual, 'settings_subset', '{}'::jsonb,
      'changed_fields', '[]'::jsonb, 'source_version', s.version)
  FROM org.schools s
  JOIN platform.tenants t ON t.tenant_id = s.tenant_id AND t.tenant_type = 'common' AND t.deleted_at IS NULL
  LEFT JOIN catalog.provinces p ON p.province_id = s.province_id
  WHERE s.is_virtual AND s.deleted_at IS NULL
  UNION ALL
  -- province.updated (op = created; platform event: tenant_id NULL) of every province of the catalog.
  SELECT 1 + row_number() OVER (ORDER BY p.code), 'province.updated', NULL::uuid, 'province', p.province_id::text,
    jsonb_build_object('province_id', p.province_id, 'code', p.code, 'country_code', p.country_code,
      'official_code', p.official_code, 'name', jsonb_build_object('vi', p.name_i18n->>'vi', 'en', p.name_i18n->>'en'),
      'status', p.status, 'op', 'created', 'successor_code', NULL, 'effective_to', p.effective_to)
  FROM catalog.provinces p
  WHERE p.deleted_at IS NULL
) r
CROSS JOIN LATERAL (SELECT (substr(c.ms, 1, 8) || '-' || substr(c.ms, 9, 4) || '-7' || lpad(to_hex(r.seq), 3, '0') || '-8'
    || substr(md5(r.typ || ':' || r.subject_id), 1, 3) || '-' || substr(md5(r.typ || ':' || r.subject_id), 4, 12))::uuid AS id) e
WHERE NOT EXISTS (SELECT 1 FROM infra.outbox_events o WHERE o.event_type = r.typ AND o.partition_key = r.subject_id);

SELECT set_config('app.tenant_id', '', true);
SELECT pg_notify('ew_outbox', '');
