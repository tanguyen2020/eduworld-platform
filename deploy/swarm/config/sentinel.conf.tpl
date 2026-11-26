# Valkey Sentinel template (F130): copied to /data/sentinel.conf at start (Sentinel rewrites its file); the auth
# lines are appended from the Docker secret valkey_sentinel_password by the entrypoint.
port 26379
sentinel resolve-hostnames yes
sentinel announce-hostnames yes
sentinel monitor cache valkey-cache 6379 2
sentinel down-after-milliseconds cache 5000
sentinel failover-timeout cache 60000
sentinel monitor ratelimit valkey-ratelimit 6379 2
sentinel down-after-milliseconds ratelimit 5000
sentinel failover-timeout ratelimit 60000
