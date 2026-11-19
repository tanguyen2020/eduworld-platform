// ESLint rule ew/no-env-url-literal (F050-AC-03, AGENTS R3): environment URLs (gateway, issuer, any
// http(s)/ws(s) origin, localhost, host:port) come from the runtime config (/config.json or VITE_*), never from
// source literals.

const URL_LITERAL = /\b(?:https?|wss?):\/\/[^\s'"`]+|\blocalhost(?::\d+)?\b|\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b/i;

export default {
  meta: {
    type: 'problem',
    docs: { description: 'Forbid environment URLs in source; read them from the runtime config (AGENTS R3)' },
    messages: { url: 'Environment URL "{{value}}" in source: read it from the runtime config (RuntimeConfig / VITE_*).' },
    schema: [{ type: 'object', properties: { allow: { type: 'array', items: { type: 'string' } } }, additionalProperties: false }],
  },
  create(context) {
    const allow = context.options[0]?.allow ?? [];
    const check = (node, text) => {
      const m = URL_LITERAL.exec(text);
      if (m && !allow.some((a) => m[0].startsWith(a))) context.report({ node, messageId: 'url', data: { value: m[0] } });
    };
    return {
      Literal(node) { if (typeof node.value === 'string') check(node, node.value); },
      TemplateElement(node) { check(node, node.value.cooked ?? node.value.raw); },
    };
  },
};
