package persistence

const ok = `SELECT id FROM content.items WHERE tenant_id = $1 AND deleted_at IS NULL`
const missing = `SELECT id FROM content.items WHERE id = $1`
const hardDelete = `DELETE FROM content.items WHERE id = $1`
const infraOK = `SELECT 1 FROM infra.outbox_events`

// tenancyguard:ok projection rebuild reads every tenant under Bypass(platform_job)
const exempt = `SELECT id FROM content.items`
