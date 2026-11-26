// GifBuilder (F082 T016, F082-AC-05, AC-19): the payload editor of one GIF of the internal library — static preview from the
// signed URL of module chat (click to play), the bilingual tags (schema bounds, counters). The files come from the
// upload of the library page; there is no URL field and no third-party GIF search anywhere (ADR-0034). Review and
// publish are the shared workflow of the editor (Approve hidden for the uploader; the server enforces reviewer ≠ author).
import { canonicalPath } from '@eduworld/ops-ui/game-console';
import { Flex, Form } from 'antd';
import { useTranslation } from 'react-i18next';
import type { BuilderProps } from '../builders/types';
import { GifTile } from './GifTile';
import { tagRules, type GifPayload } from './gif';
import { TagListInput } from './TagListInput';

export function GifBuilder({ def, value, onChange, readOnly, errors }: BuilderProps) {
  const { t } = useTranslation();
  const gif = value as GifPayload;
  const rules = tagRules(def.schema);
  const err = Object.fromEntries(Object.entries(errors).map(([k, v]) => [canonicalPath(k), v]));
  const set = (patch: Partial<GifPayload>) => onChange({ ...gif, ...patch } as Record<string, unknown>);
  const tagError = err.tags ?? err.tags_vi ?? err.tags_en;
  return (
    <Flex vertical gap="middle" data-testid="gif-builder">
      <GifTile gif={gif} />
      <Form layout="vertical" component="div">
        <TagListInput
          id="gf-gifs-tags-vi"
          label={t('gameOps.chatAssets.gif.tagsVi')}
          addLabel={t('gameOps.chatAssets.gif.addTagVi')}
          value={gif.tags_vi ?? []}
          onChange={(tags) => set({ tags_vi: tags.length ? tags : undefined })}
          readOnly={readOnly}
          error={tagError}
          {...rules}
        />
        <TagListInput
          id="gf-gifs-tags-en"
          label={t('gameOps.chatAssets.gif.tagsEn')}
          addLabel={t('gameOps.chatAssets.gif.addTagEn')}
          value={gif.tags_en ?? []}
          onChange={(tags) => set({ tags_en: tags.length ? tags : undefined })}
          readOnly={readOnly}
          {...rules}
        />
      </Form>
    </Flex>
  );
}
