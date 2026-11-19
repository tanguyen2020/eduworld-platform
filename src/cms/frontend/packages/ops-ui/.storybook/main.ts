// Storybook 8 for ops-ui (F050 T036, DESIGN_SYSTEM §9). Game imports fail here too (F050-AC-07).
import type { StorybookConfig } from '@storybook/react-vite';
import path from 'node:path';
import { forbidGameImports } from '../../../tooling/forbid-game-imports.mjs';

const REPO_ROOT = path.resolve(__dirname, '../../../../../..');

const config: StorybookConfig = {
  framework: { name: '@storybook/react-vite', options: {} },
  stories: ['../src/**/*.stories.tsx'],
  addons: ['@storybook/addon-essentials'],
  core: { disableTelemetry: true },
  async viteFinal(cfg) {
    cfg.plugins = [forbidGameImports() as never, ...(cfg.plugins ?? [])];
    cfg.server = { ...cfg.server, fs: { ...cfg.server?.fs, allow: [...(cfg.server?.fs?.allow ?? []), path.resolve(__dirname, '../../..'), path.join(REPO_ROOT, 'design/fonts')] } };
    return cfg;
  },
};

export default config;
