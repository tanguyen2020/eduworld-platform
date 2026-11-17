-- ew-tenant-header (F004 T020, research R1): format check of X-Tenant-Id at the gateway (ADR-0065).
-- mode=required: missing header → 400 TENANT_HEADER_REQUIRED; mode=optional: checked only when present.
-- Invalid UUID → 400 BAD_REQUEST. A valid header is exposed as ctx.var.ew_tenant_id (rate-limit key,
-- logs, traces). The gateway never decides tenant membership or permissions: the backend does (F002).
local core = require("apisix.core")

local plugin_name = "ew-tenant-header"
local UUID = [[^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$]]

local schema = {
    type = "object",
    properties = {
        mode = { type = "string", enum = { "required", "optional" }, default = "required" },
    },
}

local _M = {
    version = 0.1,
    priority = 2500, -- after openid-connect (2599): unauthenticated requests get 401 first
    name = plugin_name,
    schema = schema,
}

function _M.check_schema(conf)
    return core.schema.check(schema, conf)
end

local function problem(ctx, status, code, title)
    core.response.set_header("Content-Type", "application/problem+json", "Cache-Control", "no-store")
    return status, {
        type = "about:blank", title = title, status = status, code = code,
        trace_id = core.request.header(ctx, "X-Request-Id"),
    }
end

function _M.rewrite(conf, ctx)
    local value = core.request.header(ctx, "X-Tenant-Id")
    if value == nil or value == "" then
        if conf.mode == "required" then
            return problem(ctx, 400, "TENANT_HEADER_REQUIRED", "X-Tenant-Id header is required")
        end
        return
    end
    if not ngx.re.find(value, UUID, "jo") then
        return problem(ctx, 400, "BAD_REQUEST", "X-Tenant-Id must be a UUID")
    end
    ctx.var.ew_tenant_id = value
end

return _M
