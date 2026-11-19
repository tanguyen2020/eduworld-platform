// Storybook test-runner (F050-AC-19, TC-UI-006, NFR-A11Y-003): axe runs on every story; a violation of impact
// serious or critical fails the run (CI job `storybook` of web.yml). WCAG 2.1 A/AA rules.
import type { TestRunnerConfig } from '@storybook/test-runner';
import { checkA11y, injectAxe } from 'axe-playwright';

const config: TestRunnerConfig = {
  async preVisit(page) {
    await injectAxe(page);
  },
  async postVisit(page) {
    await page.waitForLoadState('networkidle');
    await checkA11y(
      page,
      '#storybook-root',
      {
        includedImpacts: ['serious', 'critical'],
        detailedReport: true,
        detailedReportOptions: { html: true },
        axeOptions: { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] } },
      },
      false,
      'v2',
    );
  },
};

export default config;
