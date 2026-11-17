-- ew-api-rate (F004 T029): api_rate_default per user (SECURITY_ARCHITECTURE §14, CONFIG_DEFAULTS
-- api_rate_default) on authenticated routes. Runs after openid-connect has verified the bearer token,
-- takes its `sub` and counts in Valkey with a fixed window (VALKEY_DESIGN: INCR
-- ew:rl:api:<sub>:<profile>:<read|write>:<window> + EXPIRE), shared by every gateway node.
-- Reads (GET/HEAD/OPTIONS) and writes have separate limits. Over the limit → 429 Problem Details
-- RATE_LIMITED with Retry-After and RateLimit-*. Valkey unavailable → allow (allow_degradation, research
-- R3) and log a warning. Never keyed by client IP (school NAT, AGENTS R30); the token is never logged.
local core = require("apisix.core")
local redis = require("resty.redis")

local plugin_name = "ew-api-rate"

local schema = {
    type = "object",
    properties = {
        name = { type = "string", default = "api" },
        read_count = { type = "integer", minimum = 1 },
        write_count = { type = "integer", minimum = 1 },
        window = { type = "integer", minimum = 1, default = 60 },
        redis_host = { type = "string", minLength = 1 },
        redis_port = { type = "integer", minimum = 1, default = 6379 },
        redis_username = { type = "string" },
        -- The password is read from the gateway environment, never written in the route file.
        redis_password_env = { type = "string", default = "EW_APISIX_VALKEY_PASSWORD" },
        redis_timeout = { type = "integer", minimum = 10, default = 200 },
        allow_degradation = { type = "boolean", default = true },
    },
    required = { "read_count", "write_count", "redis_host" },
}

local _M = {
    version = 0.1,
    priority = 1010, -- after openid-connect (2599) and ew-tenant-header (2500), before limit-count (1002)
    name = plugin_name,
    schema = schema,
}

function _M.check_schema(conf)
    return core.schema.check(schema, conf)
end

local READ = { GET = true, HEAD = true, OPTIONS = true }

-- subject returns the `sub` claim of the bearer token (signature already verified by openid-connect).
local function subject(ctx)
    local auth = core.request.header(ctx, "Authorization")
    if type(auth) ~= "string" then
        return nil
    end
    local payload = auth:match("^[Bb]earer%s+[^.]+%.([^.]+)%.")
    if not payload then
        return nil
    end
    payload = payload:gsub("-", "+"):gsub("_", "/")
    local pad = #payload % 4
    if pad > 0 then
        payload = payload .. string.rep("=", 4 - pad)
    end
    local json = ngx.decode_base64(payload)
    local claims = json and core.json.decode(json)
    if type(claims) ~= "table" or type(claims.sub) ~= "string" or claims.sub == "" or #claims.sub > 64 then
        return nil
    end
    return claims.sub
end

local function incr(conf, key, ttl)
    local red = redis:new()
    red:set_timeouts(conf.redis_timeout, conf.redis_timeout, conf.redis_timeout)
    local ok, err = red:connect(conf.redis_host, conf.redis_port)
    if not ok then
        return nil, "connect: " .. tostring(err)
    end
    local password = os.getenv(conf.redis_password_env)
    if password and password ~= "" then
        if conf.redis_username and conf.redis_username ~= "" then
            ok, err = red:auth(conf.redis_username, password)
        else
            ok, err = red:auth(password)
        end
        if not ok then
            return nil, "auth: " .. tostring(err)
        end
    end
    local n
    n, err = red:incr(key)
    if not n then
        return nil, "incr: " .. tostring(err)
    end
    if n == 1 then
        red:expire(key, ttl)
    end
    red:set_keepalive(10000, 100)
    return n
end

function _M.access(conf, ctx)
    local sub = subject(ctx)
    if not sub then
        return -- unauthenticated requests never reach here on oidc routes
    end
    local method = core.request.get_method()
    local kind, limit = "write", conf.write_count
    if READ[method] then
        kind, limit = "read", conf.read_count
    end
    local now = ngx.time()
    local slot = math.floor(now / conf.window)
    local reset = (slot + 1) * conf.window - now
    local key = "ew:rl:api:" .. sub .. ":" .. conf.name .. ":" .. kind .. ":" .. slot
    local n, err = incr(conf, key, conf.window + 1)
    if not n then
        core.log.warn("ew-api-rate: valkey unavailable (", err, ")")
        if conf.allow_degradation then
            return
        end
        return 503, { type = "about:blank", title = "Service unavailable", status = 503, code = "SERVICE_UNAVAILABLE" }
    end
    local remaining = limit - n
    if remaining < 0 then
        core.response.set_header("Content-Type", "application/problem+json", "Cache-Control", "no-store",
            "Retry-After", tostring(reset), "RateLimit-Limit", tostring(limit), "RateLimit-Remaining", "0",
            "RateLimit-Reset", tostring(reset))
        return 429, {
            type = "about:blank", title = "Too many requests", status = 429, code = "RATE_LIMITED", retryable = true,
            trace_id = core.request.header(ctx, "X-Request-Id"),
        }
    end
    core.response.set_header("RateLimit-Limit", tostring(limit), "RateLimit-Remaining", tostring(remaining),
        "RateLimit-Reset", tostring(reset))
end

return _M
