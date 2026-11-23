-- Owner rule 2026-11-23 (API_DESIGN §5): the list operations read one keyset page in SQL — WHERE (sort_key, id) > cursor
-- ORDER BY sort_key, id LIMIT n+1 — instead of reading everything and slicing in memory. These partial indexes follow the
-- ORDER BY of each keyset (live rows only, the tenant column first where the list is tenant-scoped, R8/R21):
--   listResearchAgreements   (tenant_id, created_at, agreement_id)
--   listEfficacyStudies      (created_at DESC, study_id DESC)
--   listEfficacyWaves        (study_id, window_start, wave_id)
--   listLearningExperiments  (created_at DESC, experiment_id DESC)
--   listMyWrongQuestions     (tenant_id, student_id, next_due_at, entry_id) of the active entries
-- Hand-written (explicit-SQL tables, no ent entity); expand only (AGENTS R12).

CREATE INDEX research_agreements_keyset_idx ON learning.research_agreements (tenant_id, created_at, agreement_id)
  WHERE deleted_at IS NULL;
CREATE INDEX efficacy_studies_keyset_idx ON learning.efficacy_studies (created_at DESC, study_id DESC) WHERE deleted_at IS NULL;
CREATE INDEX efficacy_waves_keyset_idx ON learning.efficacy_waves (study_id, window_start, wave_id) WHERE deleted_at IS NULL;
CREATE INDEX experiments_keyset_idx ON learning.experiments (created_at DESC, experiment_id DESC) WHERE deleted_at IS NULL;
CREATE INDEX wrong_question_entries_keyset_idx ON learning.wrong_question_entries (tenant_id, student_id, next_due_at, entry_id)
  WHERE status = 'active' AND deleted_at IS NULL;
