#!/usr/bin/env bash
# F130-AC-19, AC-20, AC-21 on staging: RB-04 rotation (a token issued before is rejected), RB-05 restore into a new
# bucket set (100 % referenced objects), optional DR drill timed against rpo_rto (RTO ≤ 4 h). Losing ONE SeaweedFS node
# needs the 3-node production cluster (staging has one storage node, ADR-0093): drilled there per BACKUP_RECOVERY §8.
#   OLD_TOKEN_FILE=<token issued before the restore>  REFERENCED=<media_assets keys file>  ACTOR=…  [DR=1]
# shellcheck source=tests/ops/staging/lib.sh
. "$(dirname "$0")/lib.sh"
: "${OLD_TOKEN_FILE:?}" "${REFERENCED:?}" "${ACTOR:?}"
check "RB-04 rotate keys + not-before, old token rejected" job --like=backup-tools --secret=ops_kc_admin_secret -- \
  rb04-keycloak-restore.sh --phase=rotate --actor="$ACTOR" --purpose="RB-04 drill" --old-token-file="$OLD_TOKEN_FILE" --yes
check "RB-05 restore, 100 % referenced objects verified" job --like=backup-tools -- rb05-object-storage-restore.sh --referenced="$REFERENCED" --yes
if [ "${DR:-}" = 1 ]; then
  t0=$(date +%s)
  check "RB-03 DR drill completed" bash "$root/scripts/ops/rb03-cluster-restore.sh" --env=staging --actor="$ACTOR" --purpose="annual DR drill" --yes
  elapsed=$(( $(date +%s) - t0 ))
  check "DR within RTO 4 h (${elapsed} s)" test "$elapsed" -le 14400
fi
finish
