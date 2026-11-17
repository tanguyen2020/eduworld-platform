#!/usr/bin/env bash
# F001 T042 / F001-AC-16, F001-AC-26 — apply or check repository settings with the GitHub CLI.
#   bash scripts/ci/apply-repo-settings.sh           # apply (repository admin token required)
#   bash scripts/ci/apply-repo-settings.sh --check   # compare current settings, exit 1 on drift
# Branch protection of main: CODEOWNERS review, no force-push/deletion, required checks of security.yml.
# Path-filtered lanes (go/specs/docs) cannot be plain required checks (a skipped workflow stays "pending");
# they are enforced by the ci-gate follow-up noted in specs/001 (Assumptions).
# Environments staging/production: required reviewers (REVIEWERS, default: repository owner).
set -euo pipefail
command -v gh >/dev/null || { echo "gh CLI is required (https://cli.github.com)" >&2; exit 1; }
REPO="${REPO:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}"
OWNER="${REPO%%/*}"
REVIEWER_ID="${REVIEWER_ID:-$(gh api "users/${REVIEWERS:-$OWNER}" -q .id)}"
REQUIRED_CHECKS='["gitleaks","govulncheck","osv-scanner","trivy-fs"]'

protection=$(cat <<JSON
{
  "required_status_checks": { "strict": true, "contexts": $REQUIRED_CHECKS },
  "enforce_admins": true,
  "required_pull_request_reviews": { "require_code_owner_reviews": true, "required_approving_review_count": 1, "dismiss_stale_reviews": true },
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_linear_history": false
}
JSON
)
environment=$(cat <<JSON
{ "reviewers": [ { "type": "User", "id": $REVIEWER_ID } ], "deployment_branch_policy": { "protected_branches": true, "custom_branch_policies": false } }
JSON
)

if [ "${1:-}" != "--check" ]; then
  gh api -X PUT "repos/$REPO/branches/main/protection" --input - <<<"$protection" >/dev/null
  for env in staging production; do
    gh api -X PUT "repos/$REPO/environments/$env" --input - <<<"$environment" >/dev/null
  done
  echo "applied: branch protection on main, environments staging/production"
  exit 0
fi

drift=0
cur=$(gh api "repos/$REPO/branches/main/protection" 2>/dev/null || echo '{}')
check() { # description jq-expression expected
  got=$(jq -r "$2" <<<"$cur")
  if [ "$got" = "$3" ]; then echo "ok   - $1"; else echo "DRIFT - $1 (got $got, want $3)"; drift=1; fi
}
check "CODEOWNERS review required" '.required_pull_request_reviews.require_code_owner_reviews // false' true
check "force-push blocked" '.allow_force_pushes.enabled // true' false
check "required checks" '[.required_status_checks.contexts[]?] | sort | join(",")' "$(jq -r 'sort | join(",")' <<<"$REQUIRED_CHECKS")"
for env in staging production; do
  n=$(gh api "repos/$REPO/environments/$env" -q '[.protection_rules[]? | select(.type=="required_reviewers")] | length' 2>/dev/null || echo 0)
  if [ "$n" -ge 1 ]; then echo "ok   - environment $env has required reviewers"; else echo "DRIFT - environment $env without required reviewers"; drift=1; fi
done
exit $drift
