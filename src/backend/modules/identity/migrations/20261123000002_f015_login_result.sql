-- F015 T025 — the login journal records refusals outside the access hours of the school/tenant
-- (ACCESS_OUTSIDE_ALLOWED_HOURS, F015-AC-18). Applied as identity_migrator.
ALTER TABLE identity.login_events DROP CONSTRAINT login_events_result_check;
ALTER TABLE identity.login_events ADD CONSTRAINT login_events_result_check CHECK (result IN ('success','invalid_credentials',
  'account_not_provisioned','game_access_student_only','account_locked','account_deactivated','no_active_enrollment',
  'tenant_not_selected','tenant_mismatch','rate_limited','queue_busy','client_version_unsupported','curfew_active','unavailable',
  'access_outside_allowed_hours'));
