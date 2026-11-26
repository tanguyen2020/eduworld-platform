#!/bin/sh
# Alertmanager entrypoint (F131 T025): renders the committed routing file (placeholders __EW_*__ for the SMTP relay,
# sender and on-call mailing list — environment values from obs.env, not secrets), checks it, then starts
# Alertmanager. Receiver URLs and the SMTP password stay in Docker secret files (url_file / *_password_file).
set -eu
src=/etc/alertmanager/alertmanager.tpl.yml
dst=/alertmanager/alertmanager.yml
for v in EW_ALERT_SMTP_SMARTHOST EW_ALERT_SMTP_FROM EW_ALERT_SMTP_USERNAME EW_ALERT_EMAIL_TO EW_ALERT_EXTERNAL_URL; do
  eval "val=\${$v:-}"
  [ -n "$val" ] || { echo "alertmanager: $v is not set" >&2; exit 1; }
  case "$val" in *'|'*|*'"'*|*\\*) echo "alertmanager: $v has a forbidden character" >&2; exit 1 ;; esac
done
sed -e "s|__EW_ALERT_SMTP_SMARTHOST__|$EW_ALERT_SMTP_SMARTHOST|g" \
    -e "s|__EW_ALERT_SMTP_FROM__|$EW_ALERT_SMTP_FROM|g" \
    -e "s|__EW_ALERT_SMTP_USERNAME__|$EW_ALERT_SMTP_USERNAME|g" \
    -e "s|__EW_ALERT_EMAIL_TO__|$EW_ALERT_EMAIL_TO|g" "$src" > "$dst"
if grep -q '__EW_' "$dst"; then echo "alertmanager: unrendered placeholder in $dst" >&2; exit 1; fi
amtool check-config "$dst" >/dev/null
exec /bin/alertmanager --config.file="$dst" --storage.path=/alertmanager \
  --web.external-url="$EW_ALERT_EXTERNAL_URL" --cluster.listen-address= "$@"
