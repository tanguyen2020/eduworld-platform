// ESLint rule ew/no-hardcoded-style (F050-AC-06, TC-UI-003, AGENTS R32, NFR-MAINT-007): no color, font,
// font size or spacing literal in TS/TSX. Values come from the theme (antd token, ops-ui) or `var(--ew-*)`.
// Files generated from design tokens are ignored in eslint.config.mjs.
// Tailwind (ADR-0098): the theme only holds token variables, so an arbitrary value in a class string
// (`bg-[#fff]`, `p-[13px]`, `[color:red]`) is the one way left to hardcode a value — it is reported unless it is a
// single `var(--ew-*)` token (`bg-[var(--ew-ops-color-sider-bg)]`).

const COLOR_IN_STRING = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(/;

/** camelCase style keys whose literal values are forbidden. */
const STYLE_KEYS = new Set([
  'color', 'background', 'backgroundColor', 'borderColor', 'borderTopColor', 'borderRightColor', 'borderBottomColor',
  'borderLeftColor', 'outlineColor', 'fill', 'stroke', 'caretColor', 'textDecorationColor', 'boxShadow',
  'font', 'fontFamily', 'fontSize', 'fontWeight', 'lineHeight', 'letterSpacing',
  'margin', 'marginTop', 'marginRight', 'marginBottom', 'marginLeft', 'marginBlock', 'marginInline',
  'marginBlockStart', 'marginBlockEnd', 'marginInlineStart', 'marginInlineEnd',
  'padding', 'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft', 'paddingBlock', 'paddingInline',
  'paddingBlockStart', 'paddingBlockEnd', 'paddingInlineStart', 'paddingInlineEnd',
  'gap', 'rowGap', 'columnGap', 'borderRadius',
]);

// geometric values (circle, full size) are not design values
const ALWAYS_OK = new Set(['inherit', 'initial', 'unset', 'revert', 'currentColor', 'currentcolor', 'transparent', 'auto', 'none', '0', 'normal', '50%', '100%']);

// utility part of one class (after the variants): `utility-[value]` or `[property:value]`, optional `!`/`-`
const TW_ARBITRARY = /^!?-?(?:[a-z][a-z0-9-]*-\[([^\]\s]+)\]|\[([a-z-]+:[^\]\s]+)\])$/;
// the only arbitrary value allowed: one token variable, optionally with a type hint (`color:var(--ew-…)`)
const TW_TOKEN_VALUE = /^(?:[a-z-]+:)?var\(--ew-[a-z0-9-]+\)$/;

/** The utility of a Tailwind class without its variants (`md:hover:p-2` → `p-2`; `[&>a]:p-2` → `p-2`). */
function utilityOf(cls) {
  let depth = 0;
  let start = 0;
  for (let i = 0; i < cls.length; i++) {
    const c = cls[i];
    if (c === '[') depth++;
    else if (c === ']') depth--;
    else if (c === ':' && depth === 0) start = i + 1;
  }
  return cls.slice(start);
}

/** Arbitrary Tailwind values in a class-like string that are not a design-token variable. */
export function tailwindArbitraryValues(text) {
  return text.split(/\s+/).filter((cls) => {
    const m = TW_ARBITRARY.exec(utilityOf(cls));
    return m !== null && !TW_TOKEN_VALUE.test(m[1] ?? m[2]);
  });
}

function keyName(prop) {
  if (prop.computed) return null;
  if (prop.key.type === 'Identifier') return prop.key.name;
  if (prop.key.type === 'Literal') return String(prop.key.value);
  return null;
}

export default {
  meta: {
    type: 'problem',
    docs: { description: 'Forbid hardcoded colors, fonts, font sizes and spacing; use design tokens (AGENTS R32)' },
    messages: {
      color: 'Hardcoded color "{{value}}": use a design token (antd theme token, domainColors or var(--ew-*)).',
      style: 'Hardcoded {{key}} "{{value}}": use a design token (antd theme token or var(--ew-*)).',
      tailwind: 'Tailwind arbitrary value "{{value}}": use a theme class mapped to the admin tokens (ADR-0098), or [var(--ew-*)].',
    },
    schema: [{ type: 'object', properties: { allow: { type: 'array', items: { type: 'string' } } }, additionalProperties: false }],
  },
  create(context) {
    const allow = new Set(context.options[0]?.allow ?? []);
    const okValue = (v) => allow.has(String(v)) || ALWAYS_OK.has(String(v)) || /^var\(--ew-[a-z0-9-]+\)$/.test(String(v));
    const reported = new Set();
    const report = (node, messageId, data) => {
      if (reported.has(node)) return;
      reported.add(node);
      context.report({ node, messageId, data });
    };
    const checkString = (node, text) => {
      if (allow.has(text)) return;
      const m = COLOR_IN_STRING.exec(text);
      if (m) report(node, 'color', { value: m[0] });
      else {
        const [arbitrary] = tailwindArbitraryValues(text);
        if (arbitrary) report(node, 'tailwind', { value: arbitrary });
      }
    };
    return {
      Literal(node) {
        if (typeof node.value === 'string') checkString(node, node.value);
      },
      TemplateElement(node) {
        checkString(node, node.value.cooked ?? node.value.raw);
      },
      Property(node) {
        const key = keyName(node);
        if (!key || !STYLE_KEYS.has(key)) return;
        const v = node.value;
        if (v.type === 'Literal' && (typeof v.value === 'number' || typeof v.value === 'string')) {
          if (v.value === 0 || okValue(v.value)) return;
          if (typeof v.value === 'string' && COLOR_IN_STRING.test(v.value)) checkString(v, v.value);
          else report(v, 'style', { key, value: String(v.value) });
        } else if (v.type === 'TemplateLiteral' && v.expressions.length === 0) {
          const text = v.quasis.map((q) => q.value.cooked).join('');
          if (!okValue(text)) report(v, 'style', { key, value: text });
        } else if (v.type === 'UnaryExpression' && v.argument.type === 'Literal' && typeof v.argument.value === 'number') {
          report(v, 'style', { key, value: `${v.operator}${v.argument.value}` });
        }
      },
    };
  },
};
