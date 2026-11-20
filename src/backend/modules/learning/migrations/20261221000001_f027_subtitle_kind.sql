-- F027 follow-up (F065 OQ, T040): MediaKind subtitle — WebVTT subtitles of video lessons (F058 Clarifications
-- [Confirmed]; CONTENT_MODEL §7). Widens the kind CHECK of content.media_assets; expand only (AGENTS R12).
ALTER TABLE content.media_assets DROP CONSTRAINT media_assets_kind_ck;
ALTER TABLE content.media_assets
  ADD CONSTRAINT media_assets_kind_ck CHECK (kind IN ('image','audio','video','model3d','document','subtitle'));
