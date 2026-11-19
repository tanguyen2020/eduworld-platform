// F050-AC-20 (TC-UI-004): every ops-ui story (Light, Dark, Compact) at 100% and at 200% zoom (viewport halved,
// device scale 2 — what the browser does at 200%) matches its reviewed baseline.
import { expect, test } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

interface StoryIndex {
  entries: Record<string, { id: string; type: string }>;
}

const index = JSON.parse(
  fs.readFileSync(path.resolve(HERE, '../../packages/ops-ui/storybook-static/index.json'), 'utf8'),
) as StoryIndex;
const stories = Object.values(index.entries).filter((e) => e.type === 'story');

test('the story index has Light, Dark and Compact stories', () => {
  for (const mode of ['light', 'dark', 'compact']) expect(stories.some((s) => s.id.endsWith(`--${mode}`))).toBe(true);
});

for (const story of stories) {
  for (const zoom of [{ name: '100', width: 1280, height: 800, scale: 1 }, { name: '200', width: 640, height: 400, scale: 2 }]) {
    test(`${story.id} @ zoom ${zoom.name}%`, async ({ browser }) => {
      const context = await browser.newContext({ viewport: { width: zoom.width, height: zoom.height }, deviceScaleFactor: zoom.scale });
      const page = await context.newPage();
      await page.goto(`/iframe.html?id=${story.id}&viewMode=story`);
      await page.waitForSelector('#storybook-root > *');
      await page.evaluate(() => document.fonts.ready);
      await page.waitForLoadState('networkidle');
      await expect(page).toHaveScreenshot(`${story.id}--zoom${zoom.name}.png`, { fullPage: true });
      // layout does not break at 200%: no horizontal scrollbar (DESIGN_SYSTEM §4.5)
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(overflow).toBeLessThanOrEqual(0);
      await context.close();
    });
  }
}
