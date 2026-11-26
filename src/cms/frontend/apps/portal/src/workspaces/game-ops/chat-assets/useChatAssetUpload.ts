// Portal wiring of the checksum upload queue for module chat (F082-AC-03): presignChatAssetUpload with the per-file intent
// (Idempotency-Key kept on retry), PUT through the runtime's plain fetch (never the token), Problem → message + code +
// trace id of THAT file. The size/type rules are the server's (AGENTS R1).
import { useChecksumUpload, type UploadError, type UploadItem } from '@eduworld/ops-ui/game-console';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { ApiError } from '../../../shared/api/middleware/problem';
import { presignChatAsset, type ItemRef } from '../api/gameConfigs';

export interface UploadedChatFile {
  assetKey: string;
  contentType: string;
  size: number;
}

/** Problem → per-file error (translated code, server detail, trace id). */
export function useDescribeUploadError() {
  const { t } = useTranslation();
  return (e: unknown): UploadError => {
    if (e instanceof ApiError) {
      const field = Object.values(e.fieldErrors)[0];
      const detail = field?.message ?? e.detail;
      return { code: e.code, message: [t(e.i18nKey), detail].filter(Boolean).join(' — '), traceId: e.traceId };
    }
    return { message: e instanceof Error ? e.message : String(e) };
  };
}

export function useChatAssetUpload(item: ItemRef, onUploaded: (file: UploadedChatFile, item: UploadItem) => void) {
  const { api, upload } = usePortalRuntime();
  const describeError = useDescribeUploadError();
  return useChecksumUpload<UploadedChatFile>({
    presign: async (i) => {
      const r = await presignChatAsset(api, item, { fileName: i.fileName, contentType: i.contentType, size: i.size, sha256: i.sha256, intent: i.intent });
      return { uploadUrl: r.upload_url, result: { assetKey: r.asset_key, contentType: i.contentType, size: i.size } };
    },
    put: (url, init) => (upload ?? fetch)(url, init),
    onUploaded: (it, res) => onUploaded(res, it),
    describeError,
  });
}
