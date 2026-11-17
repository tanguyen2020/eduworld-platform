package purge

const purge = `DELETE FROM chat.gameplay_messages WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
