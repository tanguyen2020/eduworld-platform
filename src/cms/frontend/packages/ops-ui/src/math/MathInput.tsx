// MathInput (F053-AC-16, AC-17, AC-18, ADR-0055, ADMIN_PORTAL §1.4): visual formula input with MathLive
// `<math-field>` and the Toán / Lý / Hóa virtual keyboard, plus the LaTeX source (keyboard users, fallback when the
// visual editor cannot load) and a MathJax 4 preview. Inline `\( \)` or block `\[ \]` is chosen here; the caller
// stores the delimited LaTeX. A local check error (syntax, length, depth, forbidden command) or a server error
// `422 MATH_RENDER_FAILED` for this formula (`error`) turns the field red and is announced.
import { Alert, Flex, Input, Segmented, Typography } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { keyboardLayouts } from './keyboardLayouts';
import { LATEX_MAX_LENGTH } from './mathjaxConfig';
import { MathText } from './MathText';
import { checkLatex, renderMath, type MathRenderError } from './renderMath';
import type { SpeechLanguage } from './speech';

export interface MathInputProps {
  value: string;
  onChange: (latex: string) => void;
  display: boolean;
  onDisplayChange?: (display: boolean) => void;
  /** Server error for this formula (422 MATH_RENDER_FAILED), already translated. */
  error?: string | null;
  label: string;
  language?: SpeechLanguage;
  readOnly?: boolean;
}

interface MathfieldLike extends HTMLElement {
  value: string;
  readOnly: boolean;
}

type MathliveModule = {
  MathfieldElement: { fontsDirectory: string | null; soundsDirectory: string | null };
};

/** Loads MathLive once (own chunk); its fonts come from the bundled CSS, never from a CDN (AGENTS R3). */
let mathlive: Promise<MathliveModule> | null = null;
export function loadMathlive(): Promise<MathliveModule> {
  mathlive ??= Promise.all([import('mathlive'), import('mathlive/fonts.css')]).then(([m]) => {
    const mod = m as unknown as MathliveModule;
    mod.MathfieldElement.fontsDirectory = null;
    mod.MathfieldElement.soundsDirectory = null;
    return mod;
  });
  return mathlive;
}

/** Translated message of a local formula error. */
export function mathErrorMessage(err: MathRenderError, t: (k: string, o?: Record<string, unknown>) => string): string {
  return t(`math.error.${err.code}`, { detail: err.detail ?? '', max: LATEX_MAX_LENGTH });
}

export function MathInput({ value, onChange, display, onDisplayChange, error, label, language = 'vi', readOnly = false }: MathInputProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const host = useRef<HTMLDivElement>(null);
  const field = useRef<MathfieldLike | null>(null);
  const [visual, setVisual] = useState<'loading' | 'ready' | 'unavailable'>('loading');
  const [localError, setLocalError] = useState<string | null>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    let live = true;
    loadMathlive()
      .then(() => {
        if (!live || !host.current) return;
        const kb = (window as unknown as { mathVirtualKeyboard?: { layouts: unknown } }).mathVirtualKeyboard;
        if (kb) kb.layouts = keyboardLayouts((k) => t(k));
        const mf = document.createElement('math-field') as MathfieldLike;
        mf.setAttribute('aria-label', label);
        mf.value = value;
        mf.readOnly = readOnly;
        mf.addEventListener('input', () => onChangeRef.current(mf.value));
        host.current.replaceChildren(mf);
        field.current = mf;
        setVisual('ready');
      })
      .catch(() => live && setVisual('unavailable'));
    return () => {
      live = false;
    };
    // created once; value/readOnly are synced below
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (field.current && field.current.value !== value) field.current.value = value;
    if (field.current) field.current.readOnly = readOnly;
  }, [value, readOnly]);

  useEffect(() => {
    let live = true;
    if (!value.trim()) {
      setLocalError(null);
      return;
    }
    const early = checkLatex(value);
    if (early) {
      setLocalError(mathErrorMessage(early, t));
      return;
    }
    renderMath(value, { display, language }).then(
      () => live && setLocalError(null),
      (e: MathRenderError) => live && setLocalError(mathErrorMessage(e, t)),
    );
    return () => {
      live = false;
    };
  }, [value, display, language, t]);

  const shownError = error ?? localError;
  return (
    <Flex vertical gap="small" data-testid="math-input">
      <Segmented
        value={display ? 'block' : 'inline'}
        disabled={readOnly}
        onChange={(v) => onDisplayChange?.(v === 'block')}
        options={[
          { value: 'inline', label: t('math.inline') },
          { value: 'block', label: t('math.block') },
        ]}
      />
      <div ref={host} data-visual={visual} aria-busy={visual === 'loading'} />
      {visual === 'unavailable' && <Typography.Text type="secondary">{t('math.visualUnavailable')}</Typography.Text>}
      <Input.TextArea
        aria-label={t('math.source', { label })}
        value={value}
        readOnly={readOnly}
        maxLength={LATEX_MAX_LENGTH}
        showCount
        status={shownError ? 'error' : undefined}
        aria-invalid={!!shownError}
        rows={2}
        onChange={(e) => onChange(e.target.value)}
      />
      {shownError && <Alert type="error" showIcon role="alert" message={shownError} />}
      {value.trim() && !shownError && (
        <div aria-label={t('math.preview')}>
          <MathText latex={value} display={display} language={language} />
        </div>
      )}
    </Flex>
  );
}
