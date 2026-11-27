// useDocxImport (F028-AC-15, FR-QB-011): the Word import of Content Studio. The .docx is uploaded as a `document`
// media asset and analysed by a `lessons_docx` job (useContentImport); the report is the cursor-paged list
// listContentImportItems (table rows: one page at a time, "Tải thêm"); drafts are created one question at a time with
// createDraftFromImportItem after the formulas of the question are rendered with MathJax 4 (one Idempotency-Key per
// question through the intent header, so a retried click never creates twice).
import { usePortalRuntime } from '../../../../app/runtime';
import { useSession } from '../../../../session/SessionProvider';
import type { components } from '../../../../shared/api/schema.gen';
import { INTENT_HEADER } from '../../../../shared/api/middleware/idempotency';
import type { ApiError } from '../../../../shared/api/middleware/problem';
import { fetchAllPages } from '../../../../shared/api/fetchAllPages';
import { useCursorPages } from '../../../../shared/api/useCursorPages';
import { renderDocxFormulas } from './renderDocxFormulas';

export type ImportItem = components['schemas']['ContentImportItem'];
export type ImportItemStatus = components['schemas']['ContentImportItemStatus'];

/** Rows of one page of the report table (the backend allows ≤ 100). */
export const ITEMS_PAGE_SIZE = 20;
/** Statuses a draft can be created from (data-model §2). */
export const IMPORTABLE: readonly ImportItemStatus[] = ['ready', 'warning', 'draft_failed'];

export const importItemsKey = (scope: string, importId: string, status: string) => ['content-studio', 'import-items', scope, importId, status] as const;

export interface DraftResult {
  itemNo: number;
  label: string;
  ok: boolean;
  /** Formula render code (MATH_*) or problem code of the server. */
  code?: string;
}

/** The report table: one cursor page at a time, filtered by status. */
export function useImportItems(importId: string | undefined, status: ImportItemStatus | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useCursorPages<ImportItem>(
    importItemsKey(scopeKey, importId ?? '', status ?? ''),
    (cursor) =>
      api.call(
        api.client.GET('/cms/learning/imports/{import_id}/items', {
          params: { path: { import_id: importId! }, query: { limit: ITEMS_PAGE_SIZE, ...(status ? { status } : {}), ...(cursor ? { cursor } : {}) } },
        }),
      ),
    { enabled: !!importId },
  );
}

export function useDocxDrafts() {
  const { api } = usePortalRuntime();

  /** Every question a draft can be created from (all pages of each importable status), by item_no. */
  const importable = async (importId: string): Promise<ImportItem[]> => {
    const out: ImportItem[] = [];
    for (const status of IMPORTABLE) {
      const all = await fetchAllPages<ImportItem>((q) =>
        api.call(api.client.GET('/cms/learning/imports/{import_id}/items', { params: { path: { import_id: importId }, query: { ...q, status } } })),
      );
      out.push(...all.items);
    }
    return out.sort((a, b) => a.item_no - b.item_no);
  };

  /** Renders the formulas of one question and creates its draft. */
  const createOne = async (importId: string, item: ImportItem): Promise<DraftResult> => {
    const base = { itemNo: item.item_no, label: item.source_label ?? String(item.item_no) };
    const lang = (item.question as { content_language?: string } | undefined)?.content_language === 'en' ? 'en' : 'vi';
    const math = await renderDocxFormulas(item.formulas, lang);
    if (math.error) return { ...base, ok: false, code: math.error };
    try {
      await api.call(
        api.client.POST('/cms/learning/imports/{import_id}/items/{item_no}:create-draft', {
          params: { path: { import_id: importId, item_no: item.item_no } },
          body: { math_assets: math.assets },
          headers: { [INTENT_HEADER]: `docx:${importId}:${item.item_no}` } as never,
        }),
      );
      return { ...base, ok: true };
    } catch (e) {
      // 409 CONFLICT carries IMPORT_ITEM_* on item_no; other problems their code
      const err = e as Partial<ApiError>;
      const code = err.fieldErrors?.item_no?.code ?? err.code ?? 'ERROR';
      return { ...base, ok: false, code };
    }
  };

  return { importable, createOne };
}
