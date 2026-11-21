-- F101 T009/T010 — Party Lobby and party sessions of module competition (specs/101-party-lobby-session: data-model §1;
-- DATABASE_DESIGN §2.9; MULTIPLAYER_GAMEPLAY §10; ADR-0004, ADR-0028, ADR-0041, ADR-0089). Applied by
-- `migrate --module=competition` as competition_migrator. Expand only (AGENTS R12): a new table, nullable columns, one NOT
-- NULL relaxed. Order: ent tables and added columns (entddl block, golden test modules/competition/ent/ddl_test.go),
-- then what ent cannot express: CHECKs, FKs, RLS, soft-delete only grants, the end-once trigger, _live views.

-- BEGIN entddl
CREATE TABLE multiplayer.party_activities (
  party_activity_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  party_id uuid NOT NULL,
  activity_type varchar(64) NOT NULL,
  activity_ref varchar(64) NOT NULL,
  game_mode varchar(64),
  room_id uuid NOT NULL,
  room_type varchar(64) NOT NULL,
  external_ref uuid,
  status varchar(64) NOT NULL DEFAULT 'running',
  min_members bigint NOT NULL,
  max_members bigint NOT NULL,
  allow_continue_on_disconnect boolean NOT NULL,
  started_at timestamptz NOT NULL,
  ended_at timestamptz,
  end_reason varchar(64),
  result_summary jsonb,
  PRIMARY KEY (party_activity_id)
);
CREATE UNIQUE INDEX party_activities_one_running_uq ON multiplayer.party_activities (party_id) WHERE status = 'running' AND deleted_at IS NULL;
CREATE INDEX party_activities_party_idx ON multiplayer.party_activities (tenant_id, party_id, started_at);
CREATE INDEX party_activities_room_idx ON multiplayer.party_activities (room_id);
ALTER TABLE multiplayer.parties ADD COLUMN selected_activity_type varchar(64);
ALTER TABLE multiplayer.parties ADD COLUMN selected_activity_ref varchar(64);
ALTER TABLE multiplayer.parties ADD COLUMN selected_game_mode varchar(64);
ALTER TABLE multiplayer.session_memberships ADD COLUMN party_activity_id uuid;
CREATE INDEX session_memberships_activity_idx ON multiplayer.session_memberships (party_activity_id, student_id) WHERE party_activity_id IS NOT NULL AND deleted_at IS NULL;
-- END entddl

-- ---------- multiplayer.party_activities (T): one running activity per party, ended once, no score (FR-PTY-013) ----------
ALTER TABLE multiplayer.party_activities
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT party_activities_type_chk CHECK (activity_type IN ('explore','quest','boss','learning_challenge','coop_challenge','team_battle')),
  ADD CONSTRAINT party_activities_ref_chk CHECK (activity_ref ~ '^[a-z0-9_.-]{1,64}$'),
  ADD CONSTRAINT party_activities_room_type_chk CHECK (room_type IN ('private_instance','boss')),
  ADD CONSTRAINT party_activities_status_chk CHECK (status IN ('running','ended')),
  ADD CONSTRAINT party_activities_ended_chk CHECK ((status = 'ended') = (ended_at IS NOT NULL) AND (ended_at IS NULL) = (end_reason IS NULL)),
  ADD CONSTRAINT party_activities_end_reason_chk CHECK (end_reason IS NULL OR end_reason IN ('completed','timeout','all_left','disbanded','server_failure')),
  ADD CONSTRAINT party_activities_members_chk CHECK (min_members BETWEEN 1 AND 8 AND max_members BETWEEN min_members AND 8),
  ADD CONSTRAINT party_activities_summary_chk CHECK (result_summary IS NULL OR (jsonb_typeof(result_summary) = 'object'
    AND octet_length(result_summary::text) <= 4096)),
  ADD CONSTRAINT party_activities_party_fk FOREIGN KEY (party_id) REFERENCES multiplayer.parties (party_id),
  ADD CONSTRAINT party_activities_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE multiplayer.party_activities ENABLE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.party_activities FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON multiplayer.party_activities FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON multiplayer.party_activities FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON multiplayer.party_activities FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON multiplayer.party_activities FROM competition_app, competition_platform;

CREATE FUNCTION multiplayer.party_activities_end_once() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = 'ended' AND (NEW.status IS DISTINCT FROM OLD.status OR NEW.ended_at IS DISTINCT FROM OLD.ended_at
    OR NEW.end_reason IS DISTINCT FROM OLD.end_reason) THEN
    RAISE EXCEPTION 'PARTY_ACTIVITY_ALREADY_ENDED' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER party_activities_end_once BEFORE UPDATE ON multiplayer.party_activities FOR EACH ROW
  EXECUTE FUNCTION multiplayer.party_activities_end_once();

-- ---------- multiplayer.parties: the activity chosen in the Party Lobby (durable, F101-AC-06) ----------
ALTER TABLE multiplayer.parties
  ADD CONSTRAINT parties_selected_type_chk CHECK (selected_activity_type IS NULL OR selected_activity_type IN ('explore','quest','boss',
    'learning_challenge','coop_challenge','team_battle')),
  ADD CONSTRAINT parties_selected_chk CHECK ((selected_activity_type IS NULL) = (selected_activity_ref IS NULL)
    AND (selected_game_mode IS NULL OR selected_activity_type IS NOT NULL));

-- ---------- multiplayer.rooms: summary of the boss room of a party session (room_id = encounter_id of module game) ----------
ALTER TABLE multiplayer.rooms DROP CONSTRAINT rooms_type_chk,
  ADD CONSTRAINT rooms_type_chk CHECK (room_type IN ('hub_instance','private_instance','lobby','battle','coop','boss'));

-- ---------- multiplayer.session_memberships: party sessions (game_session_id known when the student enters the room) ----------
ALTER TABLE multiplayer.session_memberships ALTER COLUMN game_session_id DROP NOT NULL;
ALTER TABLE multiplayer.session_memberships
  ADD CONSTRAINT session_memberships_activity_fk FOREIGN KEY (party_activity_id) REFERENCES multiplayer.party_activities (party_activity_id),
  ADD CONSTRAINT session_memberships_session_chk CHECK (game_session_id IS NOT NULL OR party_activity_id IS NOT NULL);

-- ---------- _live views for reporting (competition_readonly): new columns appended ----------
CREATE OR REPLACE VIEW multiplayer.party_activities_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.party_activities WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW multiplayer.parties_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.parties WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW multiplayer.session_memberships_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.session_memberships WHERE deleted_at IS NULL;
GRANT SELECT ON multiplayer.party_activities_live TO competition_readonly;
