-- F136 T007/T039 — free-text nickname of module game (specs/136-free-text-nickname: data-model §1, §3; ADR-0105;
-- CHARACTER_SYSTEM §5.3; DATABASE_DESIGN §2.8). Applied by `migrate --module=game` as game_migrator. Expand only (R12):
-- one nullable column and one partial unique index on game.game_profiles, one nullable column on the append-only
-- game.nickname_changes; the F070 index game_profiles_nickname_uq (tenant_id, lower(nickname)) stays. Order: ent columns
-- and index (entddl block, golden test modules/game/ent/ddl_test.go), backfill of the existing rows, the SECURITY DEFINER
-- anonymization of the history.

-- BEGIN entddl

ALTER TABLE game.game_profiles ADD COLUMN nickname_key varchar(32);
CREATE UNIQUE INDEX game_profiles_nickname_key_uq ON game.game_profiles (tenant_id, nickname_key) WHERE deleted_at IS NULL AND nickname_key IS NOT NULL;
ALTER TABLE game.nickname_changes ADD COLUMN source_ref_id uuid;
-- END entddl

-- ---------- backfill (expand step, existing rows) ----------
-- nickname_key = lower case of the NFC, trimmed, space-collapsed nickname (the key the application writes from now on).
-- Live rows of a tenant that fold to the same key (possible only for old rows differing in Unicode form or spaces —
-- the F070 index already forbids equal lower(nickname)) keep their nickname: the oldest row (a row already holding the
-- key first) gets the key, the others keep nickname_key NULL until their next change, so the unique index is never
-- violated and nothing is deleted or renamed. Soft-deleted rows get their key too (a restore compares keys). Runs as
-- the table owner across every tenant: FORCE ROW LEVEL SECURITY is lifted for this transaction only and restored below
-- (precedent chat 20261231000002; application roles never bypass RLS). Idempotent: a second run changes nothing
-- (TestNicknameKeyBackfill runs this block twice).
-- BEGIN backfill
ALTER TABLE game.game_profiles NO FORCE ROW LEVEL SECURITY;
WITH keyed AS (
  SELECT game_profile_id, nickname_key,
         lower(regexp_replace(btrim(normalize(nickname, NFC)), '\s+', ' ', 'g')) AS k,
         created_at, tenant_id
  FROM game.game_profiles
  WHERE deleted_at IS NULL AND nickname IS NOT NULL
), ranked AS (
  SELECT game_profile_id, k,
         row_number() OVER (PARTITION BY tenant_id, k ORDER BY (nickname_key IS DISTINCT FROM k), created_at, game_profile_id) AS rn
  FROM keyed
)
UPDATE game.game_profiles g SET nickname_key = r.k
FROM ranked r
WHERE g.game_profile_id = r.game_profile_id AND r.rn = 1 AND char_length(r.k) BETWEEN 1 AND 32
  AND g.nickname_key IS DISTINCT FROM r.k;
UPDATE game.game_profiles SET nickname_key = lower(regexp_replace(btrim(normalize(nickname, NFC)), '\s+', ' ', 'g'))
WHERE deleted_at IS NOT NULL AND nickname IS NOT NULL AND nickname_key IS NULL
  AND char_length(lower(regexp_replace(btrim(normalize(nickname, NFC)), '\s+', ' ', 'g'))) BETWEEN 1 AND 32;
ALTER TABLE game.game_profiles FORCE ROW LEVEL SECURITY;
-- END backfill

ALTER TABLE game.game_profiles
  ADD CONSTRAINT game_profiles_nickname_key_chk CHECK (nickname_key IS NULL OR (nickname IS NOT NULL AND nickname_key = lower(nickname_key)));

-- ---------- game.nickname_changes (TA): anonymization of the typed nicknames (F136-AC-31, ADR-0105 §8) ----------
-- A typed nickname may hold personal data, so anonymizeUserData replaces old_nickname/new_nickname of the student's
-- history too. The table stays append-only for the application roles (UPDATE revoked by F070): this owner function is the
-- only UPDATE path, in the tenant of the caller's session only (infra.current_tenant_id(), RLS policy below), never a
-- delete (ADR-0070). Returns the number of rows changed; the caller records the audit.
CREATE POLICY tenant_anonymize ON game.nickname_changes FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE FUNCTION game.anonymize_nickname_changes(p_tenant_id uuid, p_student_id uuid, p_value varchar(32)) RETURNS integer
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, game, infra AS $$
DECLARE
  n integer;
BEGIN
  IF p_tenant_id IS NULL OR p_tenant_id IS DISTINCT FROM infra.current_tenant_id() THEN
    RAISE EXCEPTION 'NICKNAME_ANONYMIZE_TENANT' USING ERRCODE = '42501';
  END IF;
  IF p_value IS NULL OR char_length(p_value) NOT BETWEEN 1 AND 32 THEN
    RAISE EXCEPTION 'NICKNAME_ANONYMIZE_VALUE' USING ERRCODE = '22023';
  END IF;
  UPDATE game.nickname_changes
     SET old_nickname = CASE WHEN old_nickname IS NULL OR old_nickname = '' THEN old_nickname ELSE p_value END,
         new_nickname = CASE WHEN new_nickname = '' THEN new_nickname ELSE p_value END
   WHERE tenant_id = p_tenant_id AND student_id = p_student_id
     AND ((old_nickname IS NOT NULL AND old_nickname NOT IN ('', p_value)) OR new_nickname NOT IN ('', p_value));
  GET DIAGNOSTICS n = ROW_COUNT;
  RETURN n;
END
$$;
REVOKE ALL ON FUNCTION game.anonymize_nickname_changes(uuid, uuid, varchar) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION game.anonymize_nickname_changes(uuid, uuid, varchar) TO game_app;
