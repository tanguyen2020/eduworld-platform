// GifUploadDialog (F082 T016, F082-AC-03, AC-05): upload several GIFs to the internal library. For each file, one intent
// (its Idempotency-Keys are kept on retry): createGameConfig('gifs') → static preview of the first frame drawn by the
// browser → presignChatAssetUpload + PUT of the GIF and of the preview (plain fetch, no token) → updateGameConfigDraft
// (If-Match) with the keys, size, dimensions and the tags chosen here. Module chat reads both files back at submit and
// refuses what breaks its rules (chat_asset_file_max) — no limit is decided here (AGENTS R1). No external URL, no
// third-party search (ADR-0034).
import { safeFileName, sha256Hex, UploadQueue, type UploadItem } from '@eduworld/ops-ui/game-console';
import { Alert, Flex, Form, Modal } from 'antd';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { INTENT_HEADER } from '../../../shared/api/middleware/idempotency';
import { gameKeys, presignChatAsset, saveDraft, type GameConfigVersion } from '../api/gameConfigs';
import { CONFIG_TYPES } from '../registry/configTypes';
import { TITLE_MAX } from '../editor/ConfigEditorPage';
import { gifFirstFrame } from './gifFrame';
import { tagRules } from './gif';
import { TagListInput } from './TagListInput';
import { useDescribeUploadError } from './useChatAssetUpload';

const newId = () => crypto.randomUUID();

export function GifUploadDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const { api, upload } = usePortalRuntime();
  const qc = useQueryClient();
  const describe = useDescribeUploadError();
  const def = CONFIG_TYPES.gifs;
  const rules = tagRules(def.schema);
  const [tagsVi, setTagsVi] = useState<string[]>([]);
  const [tagsEn, setTagsEn] = useState<string[]>([]);
  const [items, setItems] = useState<UploadItem[]>([]);
  const tags = useRef({ vi: tagsVi, en: tagsEn });
  tags.current = { vi: tagsVi, en: tagsEn };
  const patch = (id: string, p: Partial<UploadItem>) => setItems((list) => list.map((x) => (x.id === id ? { ...x, ...p } : x)));
  const put = (url: string, file: File) => (upload ?? fetch)(url, { method: 'PUT', body: file, headers: { 'Content-Type': file.type || 'application/octet-stream' } });

  const run = async (it: UploadItem) => {
    try {
      patch(it.id, { status: 'presigning', progress: 10, error: undefined });
      const tagPayload = { ...(tags.current.vi.length ? { tags_vi: tags.current.vi } : {}), ...(tags.current.en.length ? { tags_en: tags.current.en } : {}) };
      const created = await api.client.POST('/game-admin/{config_type}', {
        params: { path: { config_type: 'gifs' } },
        body: { title: it.name.slice(0, TITLE_MAX), payload: tagPayload },
        headers: { [INTENT_HEADER]: `${it.intent}:create` } as never,
      });
      const version = await api.call<GameConfigVersion>(Promise.resolve(created));
      const etag = created.response.headers.get('ETag');
      const ref = { configType: 'gifs' as const, configId: version.config_id };
      const frame = await gifFirstFrame(it.file);
      const send = async (file: File, step: string) => {
        const sha256 = await sha256Hex(file);
        const r = await presignChatAsset(api, ref, { fileName: safeFileName(file.name), contentType: file.type, size: file.size, sha256, intent: `${it.intent}:${step}` });
        const res = await put(r.upload_url, file);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return r.asset_key;
      };
      patch(it.id, { status: 'uploading', progress: 40 });
      const assetKey = await send(it.file, 'gif');
      patch(it.id, { progress: 70 });
      const previewKey = await send(frame.preview, 'preview');
      await saveDraft(api, ref, version.version, { title: version.title, payload: { ...tagPayload, asset_key: assetKey, preview_key: previewKey, size_bytes: it.file.size, width: frame.width, height: frame.height } }, etag);
      patch(it.id, { status: 'done', progress: 100 });
      void qc.invalidateQueries({ queryKey: gameKeys.all });
    } catch (e) {
      patch(it.id, { status: 'error', error: describe(e) });
    }
  };

  const add = (files: File[]) => {
    const added = files.map<UploadItem>((file) => ({ id: newId(), name: file.name, file, status: 'hashing', progress: 0, intent: newId() }));
    setItems((list) => [...list, ...added]);
    void (async () => {
      for (const it of added) await run(it);
    })();
  };

  return (
    <Modal open={open} title={t('gameOps.chatAssets.gif.upload')} onCancel={onClose} onOk={onClose} okText={t('gameOps.common.close')} cancelButtonProps={{ hidden: true }} destroyOnHidden>
      <Flex vertical gap="middle">
        <Alert type="info" showIcon message={t('gameOps.chatAssets.gif.uploadHint')} />
        <Form layout="vertical" component="div">
          <TagListInput id="gif-upload-tags-vi" label={t('gameOps.chatAssets.gif.tagsVi')} addLabel={t('gameOps.chatAssets.gif.addTagVi')} value={tagsVi} onChange={setTagsVi} {...rules} />
          <TagListInput id="gif-upload-tags-en" label={t('gameOps.chatAssets.gif.tagsEn')} addLabel={t('gameOps.chatAssets.gif.addTagEn')} value={tagsEn} onChange={setTagsEn} {...rules} />
          <Form.Item label={t('gameOps.chatAssets.gif.files')} htmlFor="gif-upload-files">
            <input id="gif-upload-files" type="file" multiple accept="image/gif" onChange={(e) => (add(Array.from(e.target.files ?? [])), (e.target.value = ''))} />
          </Form.Item>
        </Form>
        <UploadQueue items={items} onRetry={(id) => { const it = items.find((x) => x.id === id); if (it) void run(it); }} onRemove={(id) => setItems((l) => l.filter((x) => x.id !== id))} />
      </Flex>
    </Modal>
  );
}
