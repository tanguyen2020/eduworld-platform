package port

// AuditMetrics records eduworld_audit_records_total{service,result}, eduworld_audit_sanitized_fields_total
// and eduworld_partition_missing_total{table} (plan F015 "Quan sát").
type AuditMetrics interface {
	Recorded(service, result string)
	Sanitized(n int)
	PartitionMissing(table string)
}
