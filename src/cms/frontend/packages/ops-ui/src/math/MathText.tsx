// MathText (F053-AC-19, DESIGN_SYSTEM §4.6, TC-UI-011): shows a content text whose formulas (`\( \)`, `\[ \]`, or a
// whole `latex` value) are rendered by MathJax 4 (mhchem included) as SVG in `currentColor`, so they take the theme
// text color in light and dark mode. Each formula is an `img` with its speech text as accessible name. Text parts are
// plain React text (never HTML); the SVG comes from MathJax after the LaTeX allowlist check (renderMath). A formula
// that does not render shows its LaTeX source in the error color, never a broken box.
import { Typography } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { splitMath } from './delimiters';
import { renderMath, type RenderedMath } from './renderMath';
import type { SpeechLanguage } from './speech';

export interface MathTextProps {
  /** Text with delimiters, or a bare LaTeX formula when `latex` is set. */
  text?: string;
  latex?: string;
  display?: boolean;
  language?: SpeechLanguage;
}

function Formula({ latex, display, language }: { latex: string; display: boolean; language: SpeechLanguage }) {
  const { t } = useTranslation(OPS_UI_NS);
  const [state, setState] = useState<{ r?: RenderedMath; failed?: boolean }>({});
  useEffect(() => {
    let live = true;
    setState({});
    renderMath(latex, { display, language }).then(
      (r) => live && setState({ r }),
      () => live && setState({ failed: true }),
    );
    return () => {
      live = false;
    };
  }, [latex, display, language]);
  if (state.failed) {
    return (
      <Typography.Text type="danger" code aria-label={t('math.renderFailed')}>
        {latex}
      </Typography.Text>
    );
  }
  if (!state.r) return <Typography.Text type="secondary">{latex}</Typography.Text>;
  const Tag = display ? 'div' : 'span';
  return <Tag className="ew-math" data-display={display} role="img" aria-label={state.r.speech_text} dangerouslySetInnerHTML={{ __html: state.r.svg }} />;
}

export function MathText({ text, latex, display = false, language = 'vi' }: MathTextProps) {
  if (latex !== undefined) return <Formula latex={latex} display={display} language={language} />;
  return (
    <>
      {splitMath(text ?? '').map((s, i) =>
        s.kind === 'text' ? <span key={i}>{s.value}</span> : <Formula key={i} latex={s.latex} display={s.display} language={language} />,
      )}
    </>
  );
}
