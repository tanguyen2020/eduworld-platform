-- ew-login-key (F004 T028, research R2): rate limits of login routes without logging the body.
-- Reads the JSON body (≤ max_body), takes the login field, normalizes it (trim + lower case) and keeps
-- only its HMAC-SHA256 (key from env secret_env) in ctx.var.ew_login_user_hash. Three counters
-- (resty.limit.count on the shared dict ew-login-limit of config.yaml, per gateway node):
--   device (X-Device-Id or body device_id) → 429 RATE_LIMITED; username → 429 RATE_LIMITED;
--   global → 429 global_code (LOGIN_QUEUE_BUSY for the game) with Retry-After and jitter.
-- Never limits by client IP (school NAT, AGENTS R30). Responses are Problem Details with RateLimit-*.
local core = require("apisix.core")
local hmac = require("resty.hmac")
local str = require("resty.string")
local limit_count = require("resty.limit.count")

local plugin_name = "ew-login-key"

local schema = {
    type = "object",
    properties = {
        fields = { type = "array", items = { type = "string" }, default = { "username" } },
        device_field = { type = "string", default = "device_id" },
        secret_env = { type = "string", default = "EW_APISIX_LOGIN_KEY_SECRET" },
        max_body = { type = "integer", minimum = 256, default = 4096 },
        device_count = { type = "integer", minimum = 1, default = 10 },
        user_count = { type = "integer", minimum = 1, default = 10 },
        window = { type = "integer", minimum = 1, default = 60 },
        global_count = { type = "integer", minimum = 1, default = 120 },
        global_window = { type = "integer", minimum = 1, default = 1 },
        global_code = { type = "string", default = "LOGIN_QUEUE_BUSY" },
        dict = { type = "string", default = "ew-login-limit" },
        name = { type = "string", default = "login" },
    },
}

local _M = {
    version = 0.1,
    priority = 2450, -- after ew-tenant-header (2500), before limit-count (1002)
    name = plugin_name,
    schema = schema,
}

function _M.check_schema(conf)
    return core.schema.check(schema, conf)
end

local function limited(ctx, code, limit, window, retry)
    core.response.set_header("Content-Type", "application/problem+json", "Cache-Control", "no-store",
        "Retry-After", tostring(retry), "RateLimit-Limit", tostring(limit), "RateLimit-Remaining", "0",
        "RateLimit-Reset", tostring(retry))
    return 429, {
        type = "about:blank", title = "Too many requests", status = 429, code = code, retryable = true,
        trace_id = core.request.header(ctx, "X-Request-Id"),
    }
end

local function take(conf, key, limit, window)
    local lim, err = limit_count.new(conf.dict, limit, window)
    if not lim then
        core.log.error("ew-login-key: ", err)
        return true, limit -- fail open on the gateway; module identity and Keycloak still limit
    end
    local delay, remaining = lim:incoming(key, true)
    if not delay then
        if remaining == "rejected" then
            return false, 0
        end
        core.log.error("ew-login-key: ", remaining)
        return true, limit
    end
    return true, remaining
end

function _M.access(conf, ctx)
    local body, err = core.request.get_body(conf.max_body, ctx)
    if err then
        return 413, { type = "about:blank", title = "Request body too large", status = 413, code = "BAD_REQUEST" }
    end
    local data = body and core.json.decode(body) or nil
    local login
    if type(data) == "table" then
        for _, f in ipairs(conf.fields) do
            if type(data[f]) == "string" and data[f] ~= "" then
                login = data[f]
                break
            end
        end
    end
    local device = core.request.header(ctx, "X-Device-Id")
    if (device == nil or device == "") and type(data) == "table" and type(data[conf.device_field]) == "string" then
        device = data[conf.device_field]
    end
    local secret = os.getenv(conf.secret_env) or ""
    if login then
        login = string.lower((login:gsub("^%s+", ""):gsub("%s+$", "")))
        local h = hmac:new(secret, hmac.ALGOS.SHA256)
        ctx.var.ew_login_user_hash = str.to_hex(h:final(login))
    end
    local jitter = math.random(0, 3)
    local ok, remaining = take(conf, conf.name .. ":global", conf.global_count, conf.global_window)
    if not ok then
        return limited(ctx, conf.global_code, conf.global_count, conf.global_window, conf.global_window + jitter)
    end
    if device and device ~= "" then
        ok, remaining = take(conf, conf.name .. ":device:" .. ngx.md5(device), conf.device_count, conf.window)
        if not ok then
            return limited(ctx, "RATE_LIMITED", conf.device_count, conf.window, conf.window + jitter)
        end
    end
    if ctx.var.ew_login_user_hash then
        ok, remaining = take(conf, conf.name .. ":user:" .. ctx.var.ew_login_user_hash, conf.user_count, conf.window)
        if not ok then
            return limited(ctx, "RATE_LIMITED", conf.user_count, conf.window, conf.window + jitter)
        end
    end
    core.response.set_header("RateLimit-Limit", tostring(conf.user_count), "RateLimit-Remaining", tostring(remaining))
end

return _M
