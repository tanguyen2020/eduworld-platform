-- F082/F118 follow-up (2026-11-26) — optimistic concurrency of the Game Studio sticker/GIF versions (API_DESIGN §7,
-- specs/082-game-studio-chat-assets-translations Clarifications 2026-11-26). Applied by `migrate --module=chat` as
-- chat_migrator. Expand only (R12): one NOT NULL column with a default on chat.chat_asset_versions (platform G table, no
-- tenant data); nothing dropped or deleted. `revision` is the ETag of a version: every UPDATE of the row adds 1 (draft
-- edit, transition, publish, archive); updateGameConfigDraft/transitionGameConfigVersion/publishGameConfigVersion of
-- sticker-packs/gifs refuse an If-Match other than the current revision with 412 VERSION_MISMATCH. Existing rows start
-- at 1 (the ETag "1" the create already answered). The immutability trigger (chat.f_asset_version_immutable) is
-- unchanged: published → archived may move the revision, a published payload still never changes.

ALTER TABLE chat.chat_asset_versions ADD COLUMN revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1);
