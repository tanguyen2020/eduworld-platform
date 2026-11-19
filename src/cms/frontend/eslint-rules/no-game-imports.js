// ESLint rule ew/no-game-imports (F050-AC-07, TC-UI-010): no import/require/dynamic import of src/game/,
// design/game-tokens/ or a Godot resource from the Admin Portal.
import { isGameImport } from '../tooling/forbid-game-imports.mjs';

export default {
  meta: {
    type: 'problem',
    docs: { description: 'Forbid importing the game client, game tokens or Godot theme files (AGENTS R32, ADR-0047)' },
    messages: { gameImport: 'Import "{{source}}" points into the game (src/game, design/game-tokens or a Godot resource); the Admin Portal must not use game code or theme.' },
    schema: [],
  },
  create(context) {
    const filename = context.filename ?? context.getFilename();
    const check = (node, source) => {
      if (isGameImport(source, filename)) context.report({ node, messageId: 'gameImport', data: { source } });
    };
    return {
      ImportDeclaration(node) { check(node.source, node.source.value); },
      ExportNamedDeclaration(node) { if (node.source) check(node.source, node.source.value); },
      ExportAllDeclaration(node) { check(node.source, node.source.value); },
      ImportExpression(node) { if (node.source.type === 'Literal') check(node.source, node.source.value); },
      CallExpression(node) {
        if (node.callee.type === 'Identifier' && node.callee.name === 'require' && node.arguments[0]?.type === 'Literal') {
          check(node.arguments[0], node.arguments[0].value);
        }
      },
    };
  },
};
