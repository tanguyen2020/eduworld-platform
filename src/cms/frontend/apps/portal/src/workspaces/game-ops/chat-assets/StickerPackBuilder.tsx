// StickerPackBuilder (F082 T013, F082-AC-02, AC-03, AC-19; CHAT_SYSTEM §7.5, ADR-0034): the payload editor of a sticker pack
// of module chat in the shared ContentEditorShell — code, bilingual name (vi "Gốc" first), unlock rule chosen from the
// PUBLISHED reward rules / items only (every page, searchable — never a free id), bilingual unlock hint, the stickers
// (preview from the signed URL of module chat, alt text vi/en with counters, reorder by drag or by the up/down buttons)
// and the upload of new files (checksum queue: presignChatAssetUpload + PUT). The draft must exist before an upload (the
// presign route needs its config_id). Server field errors are shown on their field. No limit or rule decided here: the
// server checks the payload and the bytes again (AGENTS R1).
import { SearchSelect } from '@eduworld/ops-ui';
import { BilingualField, UploadQueue, canonicalPath } from '@eduworld/ops-ui/game-console';
import { Alert, Button, Card, Flex, Form, Image, Input, Radio, theme, Typography } from 'antd';
import { useRef, useState, type DragEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useAllConfigs } from '../api/gameConfigs';
import type { BuilderProps } from '../builders/types';
import { CONFIG_TYPES, nameOf } from '../registry/configTypes';
import { packMax, renumber, unlockKindOf, type Sticker, type StickerPack, type UnlockKind } from './stickerPack';
import { useChatAssetUpload } from './useChatAssetUpload';

const ID = 'gf-sticker-packs';

type LangValue = string | { vi?: string; en?: string } | undefined;

const splitLang = (v: LangValue): { vi?: string; en?: string } => (typeof v === 'string' ? { vi: v } : (v ?? {}));
const orUndefined = (s: string | undefined) => (s && s.length ? s : undefined);

export function StickerPackBuilder({ def, value, onChange, readOnly, errors, item }: BuilderProps) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language.startsWith('en') ? 'en' : 'vi';
  const { token } = theme.useToken();
  const pack = value as StickerPack;
  // the newest payload, also between two renders (several files may finish before React re-renders)
  const latest = useRef(pack);
  latest.current = pack;
  const err = Object.fromEntries(Object.entries(errors).map(([k, v]) => [canonicalPath(k), v]));
  const set = (patch: Partial<StickerPack>) => onChange({ ...pack, ...patch } as Record<string, unknown>);
  const stickers = pack.stickers ?? [];
  const setStickers = (list: Sticker[]) => set({ stickers: renumber(list) });
  const [mode, setMode] = useState<UnlockKind>(unlockKindOf(pack));
  const rules = useAllConfigs('reward-rules', 'published', mode === 'reward_rule');
  const items = useAllConfigs('items', 'published', mode === 'item');
  const [drag, setDrag] = useState<number | null>(null);
  const max = (k: string, sticker = false) => packMax(def.schema, k, sticker) ?? 0;
  const upload = useChatAssetUpload({ configType: def.type, configId: item?.configId ?? '' }, (f) => {
    const next = { ...latest.current, stickers: renumber([...(latest.current.stickers ?? []), { asset_key: f.assetKey, content_type: f.contentType, size_bytes: f.size }]) };
    latest.current = next;
    onChange(next as Record<string, unknown>);
  });

  const move = (from: number, to: number) => {
    if (to < 0 || to >= stickers.length || from === to) return;
    const next = [...stickers];
    const [x] = next.splice(from, 1);
    next.splice(to, 0, x!);
    setStickers(next);
  };
  const onDrop = (i: number) => (e: DragEvent) => {
    e.preventDefault();
    if (drag !== null) move(drag, i);
    setDrag(null);
  };

  const ruleOptions = rules.items.map((v) => ({ value: (v.payload.reward_rule_id as string | undefined) ?? v.config_id, label: String(v.payload.code ?? v.title ?? v.config_id) }));
  const itemOptions = items.items.flatMap((v) => (typeof v.payload.item_id === 'string' ? [{ value: v.payload.item_id, label: `${v.payload.item_id} — ${nameOf(CONFIG_TYPES.items, v.payload, lang)}` }] : []));

  return (
    <Flex vertical gap="middle" data-testid="sticker-pack-builder">
      <Form layout="vertical" component="div" disabled={readOnly}>
        <Form.Item label={t('gameOps.chatAssets.pack.code')} htmlFor={`${ID}-code`} required validateStatus={err.code ? 'error' : undefined} help={err.code}>
          <Input id={`${ID}-code`} maxLength={max('code')} showCount value={pack.code ?? ''} onChange={(e) => set({ code: orUndefined(e.target.value) })} />
        </Form.Item>
        <BilingualField
          id={`${ID}-name`}
          label={t('gameOps.chatAssets.pack.name')}
          contentLanguage="vi"
          required
          readOnly={readOnly}
          maxLength={max('name_vi')}
          value={{ vi: pack.name_vi, en: pack.name_en }}
          error={err.name_vi ?? err.name_en}
          onChange={(v) => {
            const m = splitLang(v as LangValue);
            set({ name_vi: orUndefined(m.vi), name_en: orUndefined(m.en) });
          }}
        />
        <Form.Item label={t('gameOps.chatAssets.pack.unlock')} validateStatus={err.unlock_rule ? 'error' : undefined} help={err.unlock_rule}>
          <Flex vertical gap="small">
            <Radio.Group
              value={mode}
              onChange={(e) => {
                const m = e.target.value as UnlockKind;
                setMode(m);
                set({ unlock_rule: undefined });
              }}
              options={[
                { value: 'default', label: t('gameOps.chatAssets.unlock.defaultLong') },
                { value: 'reward_rule', label: t('gameOps.chatAssets.unlock.rewardRuleLong') },
                { value: 'item', label: t('gameOps.chatAssets.unlock.itemLong') },
              ]}
            />
            {mode === 'reward_rule' && (
              <SearchSelect
                aria-label={t('gameOps.chatAssets.pack.rewardRule')}
                placeholder={t('gameOps.chatAssets.pack.rewardRule')}
                loading={rules.isPending}
                truncated={rules.truncated}
                disabled={readOnly}
                value={pack.unlock_rule?.reward_rule_id}
                onChange={(v) => set({ unlock_rule: v ? { reward_rule_id: String(v) } : undefined })}
                options={ruleOptions}
              />
            )}
            {mode === 'item' && (
              <SearchSelect
                aria-label={t('gameOps.chatAssets.pack.item')}
                placeholder={t('gameOps.chatAssets.pack.item')}
                loading={items.isPending}
                truncated={items.truncated}
                disabled={readOnly}
                value={pack.unlock_rule?.item_id}
                onChange={(v) => set({ unlock_rule: v ? { item_id: String(v) } : undefined })}
                options={itemOptions}
              />
            )}
          </Flex>
        </Form.Item>
        {mode !== 'default' && (
          <BilingualField
            id={`${ID}-hint`}
            label={t('gameOps.chatAssets.pack.hint')}
            contentLanguage="vi"
            multiline
            readOnly={readOnly}
            maxLength={max('unlock_hint_vi')}
            value={{ vi: pack.unlock_hint_vi, en: pack.unlock_hint_en }}
            onChange={(v) => {
              const m = splitLang(v as LangValue);
              set({ unlock_hint_vi: orUndefined(m.vi), unlock_hint_en: orUndefined(m.en) });
            }}
          />
        )}
      </Form>

      <Card size="small" title={t('gameOps.chatAssets.pack.stickers', { count: stickers.length })}>
        <Flex vertical gap="small">
          {err.stickers && <Typography.Text type="danger">{err.stickers}</Typography.Text>}
          {stickers.length === 0 && <Typography.Text type="secondary">{t('gameOps.chatAssets.pack.noStickers')}</Typography.Text>}
          {stickers.map((s, i) => {
            const altError = err[`stickers/${i}/alt_text_vi`] ?? err[`stickers/${i}/alt_text_en`] ?? (!s.alt_text_vi ? err['stickers/alt_text_vi'] : undefined);
            return (
              <Card
                key={s.sticker_id ?? s.asset_key}
                size="small"
                type="inner"
                draggable={!readOnly}
                onDragStart={() => setDrag(i)}
                onDragOver={(e) => e.preventDefault()}
                onDrop={onDrop(i)}
                data-testid="sticker-card"
                title={t('gameOps.chatAssets.pack.sticker', { n: i + 1 })}
                extra={
                  readOnly ? null : (
                    <Flex gap="small">
                      <Button size="small" aria-label={t('gameOps.chatAssets.pack.moveUp', { n: i + 1 })} disabled={i === 0} onClick={() => move(i, i - 1)}>
                        {t('gameOps.chatAssets.pack.up')}
                      </Button>
                      <Button size="small" aria-label={t('gameOps.chatAssets.pack.moveDown', { n: i + 1 })} disabled={i === stickers.length - 1} onClick={() => move(i, i + 1)}>
                        {t('gameOps.chatAssets.pack.down')}
                      </Button>
                      <Button size="small" danger aria-label={t('gameOps.chatAssets.pack.remove', { n: i + 1 })} onClick={() => setStickers(stickers.filter((_, j) => j !== i))}>
                        {t('gameOps.chatAssets.pack.removeShort')}
                      </Button>
                    </Flex>
                  )
                }
              >
                <Flex gap="middle" align="flex-start" wrap>
                  {s.preview_url ? (
                    <Image src={s.preview_url} alt={s.alt_text_vi || t('gameOps.chatAssets.pack.sticker', { n: i + 1 })} width={token.controlHeightLG * 2} preview={false} />
                  ) : (
                    <Typography.Text type="secondary">{t('gameOps.chatAssets.pack.previewAfterSave')}</Typography.Text>
                  )}
                  <Flex vertical flex={1}>
                    <BilingualField
                      id={`${ID}-sticker-${i}`}
                      label={t('gameOps.chatAssets.pack.stickerAlt', { n: i + 1 })}
                      contentLanguage="vi"
                      required
                      readOnly={readOnly}
                      maxLength={max('alt_text_vi', true)}
                      value={{ vi: s.alt_text_vi, en: s.alt_text_en }}
                      error={altError}
                      onChange={(v) => {
                        const m = splitLang(v as LangValue);
                        setStickers(stickers.map((x, j) => (j === i ? { ...x, alt_text_vi: orUndefined(m.vi), alt_text_en: orUndefined(m.en) } : x)));
                      }}
                    />
                  </Flex>
                </Flex>
              </Card>
            );
          })}
          {!readOnly &&
            (item ? (
              <Flex vertical gap="small">
                <label htmlFor={`${ID}-upload`}>
                  <Typography.Text strong>{t('gameOps.chatAssets.pack.upload')}</Typography.Text>
                </label>
                <input id={`${ID}-upload`} type="file" multiple accept="image/webp,image/png" onChange={(e) => (upload.add(Array.from(e.target.files ?? [])), (e.target.value = ''))} />
                <Typography.Text type="secondary">{t('gameOps.chatAssets.pack.uploadHint')}</Typography.Text>
                <UploadQueue items={upload.items} onRetry={upload.retry} onRemove={upload.remove} />
              </Flex>
            ) : (
              <Alert type="info" showIcon message={t('gameOps.chatAssets.pack.saveFirst')} />
            ))}
        </Flex>
      </Card>
    </Flex>
  );
}
