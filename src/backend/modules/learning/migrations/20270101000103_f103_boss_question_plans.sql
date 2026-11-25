-- F103 T006 — question plans of the boss rounds of module game (specs/103-boss-coop-quest research R1): module game asks
-- createQuestionPlan with mode boss (context boss_encounter, attempt mode challenge, the questions of the published
-- activity of the boss requirement). Expand only: the CHECK of the plan modes accepts boss; the match_id column holds
-- the idempotency key of the plan that module game derives (UUIDv5 of encounter, phase, round, student, requirement).
-- assessment.attempts already accepts context_type boss_encounter and mode challenge (F030, ADR-0095 narrow check).
ALTER TABLE assessment.question_plans DROP CONSTRAINT question_plans_mode_ck,
  ADD CONSTRAINT question_plans_mode_ck CHECK (mode IN ('battle_1v1','team_battle','coop_challenge','timed_quiz','tournament','boss'));
