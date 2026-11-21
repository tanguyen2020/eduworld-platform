-- F105 T008 — battle engine of module competition (specs/105-battle-engine/data-model.md; DATABASE_DESIGN §2.9, §2.10,
-- §2.16; ADR-0004, ADR-0016, ADR-0028, ADR-0041, ADR-0043). Applied by `migrate --module=competition` as
-- competition_migrator. Expand only (AGENTS R12): new table competition.match_results, new columns of matches,
-- match_participants and rooms, widened CHECKs. The live state of a match room lives in the memory of its room host and
-- its checkpoint in Valkey (ew:rt:room:<room_id>:ckpt); results, participants and scores are durable here (R9).

-- BEGIN entddl
CREATE TABLE competition.match_results (
  match_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  status varchar(64) NOT NULL,
  end_reason varchar(64) NOT NULL,
  winner_team smallint,
  rated boolean NOT NULL,
  rule_version_id uuid NOT NULL,
  summary jsonb NOT NULL,
  verified_at timestamptz NOT NULL,
  recorded_by varchar(64) NOT NULL,
  PRIMARY KEY (match_id)
);
ALTER TABLE competition.matches ADD COLUMN question_plan_id uuid;
ALTER TABLE competition.matches ADD COLUMN question_plan_version varchar(64);
ALTER TABLE competition.matches ADD COLUMN room_id uuid;
ALTER TABLE competition.matches ADD COLUMN end_reason varchar(64);
ALTER TABLE competition.match_participants ADD COLUMN correct_count smallint;
ALTER TABLE competition.match_participants ADD COLUMN disconnects smallint NOT NULL DEFAULT 0;
ALTER TABLE multiplayer.rooms ADD COLUMN match_id uuid;
-- END entddl

-- ---------- competition.matches: why the match ended (data-model §1) ----------
ALTER TABLE competition.matches
  ADD CONSTRAINT matches_end_reason_chk CHECK (end_reason IS NULL OR end_reason IN ('finished','abandon','forfeit','all_left',
    'server_failure','plan_failure')),
  ADD CONSTRAINT matches_ended_chk CHECK (status NOT IN ('completed','cancelled','aborted') OR (ended_at IS NOT NULL AND end_reason IS NOT NULL));

-- ---------- competition.match_participants: outcomes of F105 ----------
ALTER TABLE competition.match_participants DROP CONSTRAINT match_participants_outcome_chk,
  ADD CONSTRAINT match_participants_outcome_chk CHECK (outcome IS NULL OR outcome IN ('win','loss','draw','abandoned','forfeit','no_contest')),
  ADD CONSTRAINT match_participants_correct_chk CHECK (correct_count IS NULL OR correct_count >= 0),
  ADD CONSTRAINT match_participants_disconnects_chk CHECK (disconnects >= 0),
  ADD CONSTRAINT match_participants_score_chk CHECK (score IS NULL OR score >= 0);

-- ---------- competition.match_results (G*: one result per match, reached through match_participants) ----------
ALTER TABLE competition.match_results
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD COLUMN tenant_ids uuid[] NOT NULL,
  ADD CONSTRAINT match_results_tenant_ids_chk CHECK (cardinality(tenant_ids) BETWEEN 1 AND 64),
  ADD CONSTRAINT match_results_status_chk CHECK (status IN ('completed','cancelled')),
  ADD CONSTRAINT match_results_end_reason_chk CHECK (end_reason IN ('finished','abandon','forfeit','all_left')),
  ADD CONSTRAINT match_results_winner_chk CHECK (winner_team IS NULL OR winner_team BETWEEN 0 AND 1),
  ADD CONSTRAINT match_results_rated_chk CHECK (NOT rated OR status = 'completed'),
  ADD CONSTRAINT match_results_summary_chk CHECK (jsonb_typeof(summary) = 'object' AND octet_length(summary::text) <= 65536),
  ADD CONSTRAINT match_results_recorded_by_chk CHECK (length(recorded_by) BETWEEN 1 AND 64),
  ADD CONSTRAINT match_results_match_fk FOREIGN KEY (match_id) REFERENCES competition.matches (match_id);
CREATE INDEX match_results_tenant_ids_idx ON competition.match_results USING gin (tenant_ids);
REVOKE DELETE, TRUNCATE ON competition.match_results FROM competition_app, competition_platform;

-- ---------- multiplayer.rooms: the match of a battle/coop room ----------
CREATE INDEX rooms_match_idx ON multiplayer.rooms (match_id) WHERE match_id IS NOT NULL AND deleted_at IS NULL;

-- ---------- competition.integrity_signals: the answer signals of the battle engine (ANTI_CHEAT §3, §6) ----------
ALTER TABLE competition.integrity_signals DROP CONSTRAINT integrity_signals_signal_type_check;
ALTER TABLE competition.integrity_signals ADD CONSTRAINT integrity_signals_signal_type_check CHECK (signal_type IN ('hub_speed',
  'hub_bounds','fast_answer','answer_before_shown','answer_closed_question','win_trading','device_sharing'));

-- ---------- _live views for reporting (competition_readonly): new columns appended ----------
CREATE OR REPLACE VIEW competition.matches_live WITH (security_barrier = true) AS
  SELECT * FROM competition.matches WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW competition.match_participants_live WITH (security_barrier = true) AS
  SELECT * FROM competition.match_participants WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW multiplayer.rooms_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.rooms WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW competition.match_results_live WITH (security_barrier = true) AS
  SELECT * FROM competition.match_results WHERE deleted_at IS NULL;
GRANT SELECT ON competition.match_results_live TO competition_readonly;
