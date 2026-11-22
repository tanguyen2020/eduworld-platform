// ESLint rule ew/no-single-page-list (owner rule 2026-11-22, ADMIN_PORTAL §1.6, API_DESIGN §5): a list is never read
// with one big page. Backend lists are cursor paginated (limit ≤ 100): tables page with the cursor ("Tải thêm",
// useCursorPages/useCursorList), dropdowns walk every page with fetchAllPages. A literal `limit` ≥ 100 in an object is
// the old "one call with limit: 100 treated as complete" bug (or a contract violation above 100), and a hand-written
// loop over `next_cursor` duplicates fetchAllPages.

const MAX_LITERAL_LIMIT = 99;

export default {
  meta: {
    type: 'problem',
    docs: { description: 'Forbid single-page list loads (limit ≥ 100) and hand-written cursor loops; use fetchAllPages or cursor pages' },
    messages: {
      bigLimit:
        'limit: {{value}} reads a list as one page. Dropdown: fetchAllPages/useAllPages (every cursor page). Table: useCursorPages with "Tải thêm" (default page size).',
      cursorLoop: 'Hand-written loop over next_cursor: use fetchAllPages (shared/api/fetchAllPages.ts) for dropdown data.',
    },
    schema: [],
  },
  create(context) {
    const file = context.filename ?? context.getFilename();
    if (/shared[\\/]api[\\/]fetchAllPages\.ts$/.test(file)) return {};
    const isLimitKey = (p) => (p.key.type === 'Identifier' && p.key.name === 'limit') || (p.key.type === 'Literal' && p.key.value === 'limit');
    const loopUsesCursor = (node) => context.sourceCode.getText(node).includes('next_cursor');
    return {
      Property(node) {
        if (!isLimitKey(node) || node.value.type !== 'Literal' || typeof node.value.value !== 'number') return;
        if (node.value.value > MAX_LITERAL_LIMIT) context.report({ node, messageId: 'bigLimit', data: { value: String(node.value.value) } });
      },
      WhileStatement(node) { if (loopUsesCursor(node)) context.report({ node, messageId: 'cursorLoop' }); },
      DoWhileStatement(node) { if (loopUsesCursor(node)) context.report({ node, messageId: 'cursorLoop' }); },
      ForStatement(node) { if (loopUsesCursor(node)) context.report({ node, messageId: 'cursorLoop' }); },
    };
  },
};
