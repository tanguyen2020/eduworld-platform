// SensitiveTermTester (F122 T007, F122-AC-08, MODERATION §11.4, DESIGN_SYSTEM §4.3): the "Thử chuỗi" panel of the
// sensitive-term catalogue. The caller sends the text to testSensitiveTerms and passes the result back; the panel shows the
// decision, the masked text and the matched categories, and highlights the matches ONLY on the text the user typed
// (`start`/`end` of the result, F122 edge case) — never a catalogue term. Nothing is logged or stored (R14, R15).
import { Alert, Button, Descriptions, Flex, Form, Input, Tag, Typography, theme } from 'antd';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { openapiInputProps } from '../forms/CharCounter';
import { OPS_UI_NS } from '../i18n';

export interface SensitiveTermMatch {
  category_code: string;
  severity: string;
  action: string;
  start: number;
  end: number;
}

export interface SensitiveTermTestOutcome {
  decision: string;
  masked_text?: string;
  matches: SensitiveTermMatch[];
}

export interface SensitiveTermTesterProps {
  /** maxLength of SensitiveTermTestRequest.text (OpenAPI). */
  maxLength: number;
  loading?: boolean;
  /** Result of the last test, with the text it was run on. */
  result?: { text: string; outcome: SensitiveTermTestOutcome } | null;
  /** Error area (translated by the caller). */
  error?: ReactNode;
  /** Extra fields of the request (target tenant…), rendered above the text. */
  extra?: ReactNode;
  onTest: (text: string) => void;
}

/** Splits the typed text into plain and matched parts (overlapping/out-of-range matches are clamped). */
export function highlightParts(text: string, matches: readonly Pick<SensitiveTermMatch, 'start' | 'end'>[]): { text: string; hit: boolean }[] {
  const marks = new Array<boolean>(text.length).fill(false);
  for (const m of matches) for (let i = Math.max(0, m.start); i < Math.min(text.length, m.end); i++) marks[i] = true;
  const out: { text: string; hit: boolean }[] = [];
  for (let i = 0; i < text.length; i++) {
    const last = out[out.length - 1];
    if (last && last.hit === marks[i]) last.text += text[i];
    else out.push({ text: text[i]!, hit: marks[i]! });
  }
  return out;
}

export function SensitiveTermTester({ maxLength, loading, result, error, extra, onTest }: SensitiveTermTesterProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const { token } = theme.useToken();
  const [text, setText] = useState('');
  return (
    <Flex vertical gap="small">
      <Form layout="vertical" onFinish={() => (text.trim() ? onTest(text) : undefined)}>
        {extra}
        <Form.Item label={t('chatSafety.tester.input')} htmlFor="ew-term-tester">
          <Input.TextArea id="ew-term-tester" value={text} autoSize={{ minRows: 2, maxRows: 4 }} {...openapiInputProps({ maxLength })} onChange={(e) => setText(e.target.value)} />
        </Form.Item>
        <Button htmlType="submit" type="primary" loading={loading} disabled={!text.trim()}>
          {t('chatSafety.tester.run')}
        </Button>
      </Form>
      {error}
      {result ? (
        <section aria-label={t('chatSafety.tester.result')}>
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label={t('chatSafety.tester.decision')}>
              <Tag data-decision={result.outcome.decision}>{t(`chatSafety.decision.${result.outcome.decision}`, { defaultValue: result.outcome.decision })}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('chatSafety.tester.highlighted')}>
              <Typography.Text data-testid="tester-highlight">
                {highlightParts(result.text, result.outcome.matches).map((p, i) =>
                  p.hit ? (
                    <mark key={i} style={{ background: token.colorWarningBg, color: token.colorText }}>
                      {p.text}
                    </mark>
                  ) : (
                    <span key={i}>{p.text}</span>
                  ),
                )}
              </Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={t('chatSafety.tester.masked')}>
              <Typography.Text data-testid="tester-masked">{result.outcome.masked_text ?? ''}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={t('chatSafety.tester.matches')}>
              {result.outcome.matches.length === 0 ? (
                <Alert type="success" showIcon message={t('chatSafety.tester.noMatch')} />
              ) : (
                <Flex wrap gap="small">
                  {result.outcome.matches.map((m, i) => (
                    <Tag key={i} data-category={m.category_code}>
                      {t('chatSafety.tester.match', { category: m.category_code, severity: m.severity, action: t(`chatSafety.decision.${m.action}`, { defaultValue: m.action }) })}
                    </Tag>
                  ))}
                </Flex>
              )}
            </Descriptions.Item>
          </Descriptions>
        </section>
      ) : null}
    </Flex>
  );
}
