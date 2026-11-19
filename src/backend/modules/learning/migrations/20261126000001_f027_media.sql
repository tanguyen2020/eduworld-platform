-- F027 — media pipeline, formula SVGs (specs/027-media-math-preview: data-model §1–§3; CONTENT_MODEL §7, §9.1;
-- DATABASE_DESIGN §2.6a, §3.13; ADR-0041, ADR-0044, ADR-0055, ADR-0089). Applied by `migrate --module=learning` as
-- learning_migrator. Order: ent tables (entddl block, golden test modules/learning/ent/ddl_test.go), then CHECKs, FKs,
-- the unique index per scope and sha256, RLS, _live views and grants. Files are never stored here (R31).

-- BEGIN entddl
CREATE TABLE content.media_assets (
  asset_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  kind varchar(16) NOT NULL,
  original_filename varchar(255) NOT NULL,
  mime_type varchar(127) NOT NULL,
  size_bytes bigint NOT NULL,
  width integer,
  height integer,
  duration_ms integer,
  sha256 varchar(64) NOT NULL,
  bucket varchar(64) NOT NULL,
  variants jsonb NOT NULL,
  alt_text varchar(500),
  license varchar(255) NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'quarantined',
  reject_reason varchar(255),
  quarantine_key varchar(512) NOT NULL,
  is_exam_material boolean NOT NULL DEFAULT false,
  upload_completed_at timestamptz,
  processed_at timestamptz,
  uploaded_by uuid NOT NULL,
  PRIMARY KEY (asset_id)
);
CREATE INDEX media_assets_library_idx ON content.media_assets (owner_scope, tenant_id, kind, status) WHERE deleted_at IS NULL;

CREATE TABLE content.media_usages (
  media_usage_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  asset_id uuid NOT NULL,
  item_type varchar(64) NOT NULL,
  item_id uuid NOT NULL,
  version_id uuid NOT NULL,
  version_no bigint NOT NULL,
  PRIMARY KEY (media_usage_id)
);
CREATE UNIQUE INDEX media_usages_uq ON content.media_usages (asset_id, item_type, version_id) WHERE deleted_at IS NULL;
CREATE INDEX media_usages_version_idx ON content.media_usages (version_id) WHERE deleted_at IS NULL;

CREATE TABLE content.math_svgs (
  sha256 varchar(64) NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  storage_key varchar(128) NOT NULL,
  latex varchar(2000) NOT NULL,
  speech_text varchar(1000),
  size_bytes integer NOT NULL,
  sanitized_at timestamptz NOT NULL,
  PRIMARY KEY (sha256)
);
-- END entddl

ALTER TABLE content.media_assets
  ADD CONSTRAINT media_assets_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT media_assets_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT media_assets_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT media_assets_kind_ck CHECK (kind IN ('image','audio','video','model3d','document')),
  ADD CONSTRAINT media_assets_status_ck CHECK (status IN ('quarantined','processing','ready','rejected')),
  ADD CONSTRAINT media_assets_bucket_ck CHECK (bucket IN ('ew-media-public','ew-media-private')),
  ADD CONSTRAINT media_assets_private_ck CHECK (bucket = 'ew-media-private' OR (owner_scope = 'platform' AND NOT is_exam_material)),
  ADD CONSTRAINT media_assets_size_ck CHECK (size_bytes > 0),
  ADD CONSTRAINT media_assets_dims_ck CHECK ((width IS NULL OR width > 0) AND (height IS NULL OR height > 0)
    AND (duration_ms IS NULL OR duration_ms > 0)),
  ADD CONSTRAINT media_assets_sha_ck CHECK (sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT media_assets_variants_ck CHECK (jsonb_typeof(variants) = 'object' AND octet_length(variants::text) <= 16384),
  ADD CONSTRAINT media_assets_alt_ck CHECK (kind <> 'image' OR alt_text IS NOT NULL),
  ADD CONSTRAINT media_assets_reject_ck CHECK ((status = 'rejected') = (reject_reason IS NOT NULL)
    AND (reject_reason IS NULL OR reject_reason IN ('virus','type_mismatch','too_large','transcode_failed')));
-- Same file in the same scope → the existing asset (duplicate_of, F027-AC-01); two tenants keep two assets.
CREATE UNIQUE INDEX media_assets_sha_scope_uq
  ON content.media_assets (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), sha256)
  WHERE deleted_at IS NULL;

ALTER TABLE content.media_usages
  ADD CONSTRAINT media_usages_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT media_usages_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT media_usages_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT media_usages_asset_fk FOREIGN KEY (asset_id) REFERENCES content.media_assets (asset_id),
  ADD CONSTRAINT media_usages_item_type_ck CHECK (item_type IN ('question','question_set','lesson','learning_material',
    'assessment_blueprint','activity','learning_path')),
  ADD CONSTRAINT media_usages_version_no_ck CHECK (version_no > 0);

ALTER TABLE content.math_svgs
  ADD CONSTRAINT math_svgs_sha_ck CHECK (sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT math_svgs_key_ck CHECK (storage_key = 'math/' || sha256 || '.svg'),
  ADD CONSTRAINT math_svgs_size_ck CHECK (size_bytes BETWEEN 1 AND 262144);
-- A formula SVG is immutable once stored (ADR-0055 §9, F027-AC-15): applications only insert.
REVOKE UPDATE ON content.math_svgs FROM learning_app, learning_platform;

-- RLS H (ADR-0089 §1): tenant rows, or platform rows whose asset is assigned to the session tenant.
ALTER TABLE content.media_assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.media_assets FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.media_assets FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'media' AND a.resource_id = media_assets.asset_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.media_assets FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.media_assets FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.media_usages ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.media_usages FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.media_usages FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'media' AND a.resource_id = media_usages.asset_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.media_usages FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.media_usages FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- Second layer for deleted rows (ADR-0089 §1).
CREATE OR REPLACE VIEW content.media_assets_live WITH (security_barrier = true) AS
  SELECT * FROM content.media_assets WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.media_usages_live WITH (security_barrier = true) AS
  SELECT * FROM content.media_usages WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.math_svgs_live WITH (security_barrier = true) AS
  SELECT * FROM content.math_svgs WHERE deleted_at IS NULL;
GRANT SELECT ON content.media_assets_live, content.media_usages_live, content.math_svgs_live TO learning_readonly;
REVOKE ALL ON content.media_assets, content.media_usages, content.math_svgs FROM learning_readonly;
