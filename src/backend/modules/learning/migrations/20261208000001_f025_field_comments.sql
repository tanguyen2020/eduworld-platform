-- F025 T046 — per-field review comments (OQ F054-1, owner 2026-11-20; CONTENT_MODEL §6): a reject may carry
-- field_comments [{path, comment}] (≤ 50, path JSON pointer ≤ 256, comment ≤ 1000), kept on the transition (history of
-- the version) and on the review task (content.review_tasks.comments, F025). Applied by migrate --module=learning as
-- learning_migrator. Expand only (AGENTS R12): the column is nullable, earlier transitions keep NULL.

-- BEGIN entddl

ALTER TABLE content.content_transitions ADD COLUMN field_comments jsonb;
-- END entddl

-- Only a reject carries comments; a JSON array of at most 50 entries and 64 KiB (the item limits are checked by the
-- use case, domain workflow.CheckFieldComments).
ALTER TABLE content.content_transitions
  ADD CONSTRAINT content_transitions_field_comments_ck CHECK (field_comments IS NULL OR (action = 'reject'
    AND jsonb_typeof(field_comments) = 'array' AND jsonb_array_length(field_comments) BETWEEN 1 AND 50
    AND octet_length(field_comments::text) <= 65536));
