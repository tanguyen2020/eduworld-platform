// MathInsertButton (F053-AC-16): "∑ Công thức" next to a rich text field → dialog with MathInput; OK inserts the
// formula at the caret as `\( … \)` (inline) or `\[ … \]` (block). The formula must pass the local check first.
import { Button, Modal } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { wrapLatex } from './delimiters';
import { MathInput } from './MathInput';
import { checkLatex } from './renderMath';
import type { SpeechLanguage } from './speech';

export interface MathInsertButtonProps {
  onInsert: (delimitedLatex: string) => void;
  language?: SpeechLanguage;
  disabled?: boolean;
}

export function MathInsertButton({ onInsert, language = 'vi', disabled }: MathInsertButtonProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const [open, setOpen] = useState(false);
  const [latex, setLatex] = useState('');
  const [display, setDisplay] = useState(false);
  const invalid = !latex.trim() || !!checkLatex(latex);
  return (
    <>
      <Button size="small" disabled={disabled} onClick={() => setOpen(true)}>
        {t('math.insert')}
      </Button>
      <Modal
        open={open}
        title={t('math.insertTitle')}
        okText={t('math.insertOk')}
        cancelText={t('math.cancel')}
        okButtonProps={{ disabled: invalid }}
        destroyOnHidden
        onCancel={() => setOpen(false)}
        onOk={() => {
          onInsert(wrapLatex(latex.trim(), display));
          setLatex('');
          setOpen(false);
        }}
      >
        <MathInput label={t('math.formula')} value={latex} onChange={setLatex} display={display} onDisplayChange={setDisplay} language={language} />
      </Modal>
    </>
  );
}
