// F053 T046 — F053-AC-27: a topic move is shown at once and sent as updateTopic (parent_topic_id, sort_order,
// If-Match W/"<row_version>"); when the server refuses, the tree goes back and the error is kept for display.
import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { problem } from '../../../test/mock-backend';
import { toApiError } from '../../../shared/api/middleware/problem';
import { useOptimisticTopics } from './TopicTree';
import { ifMatchOf, type Topic } from './useCurriculum';

const tp = (id: string, sort: number): Topic => ({ topic_id: id, name: id, subject_id: 's', grade_level: 3, sort_order: sort });
const topics = [tp('a', 1), tp('b', 2)];
const order = (list: readonly Topic[]) => [...list].sort((x, y) => (x.sort_order ?? 0) - (y.sort_order ?? 0)).map((t) => t.topic_id);

describe('useOptimisticTopics (F053-AC-27)', () => {
  it('shows the move at once and keeps it when the server accepts', async () => {
    const onMove = vi.fn(async () => undefined);
    const { result } = renderHook(() => useOptimisticTopics(topics, onMove));
    await act(() => result.current.move(topics[1]!, { parent_topic_id: null, sort_order: 1 }));
    expect(onMove).toHaveBeenCalledWith(topics[1], { parent_topic_id: null, sort_order: 1 });
    expect(order(result.current.local)).toEqual(['b', 'a']);
    expect(result.current.error).toBeNull();
  });

  it('reverts and keeps the error when the server refuses (412)', async () => {
    const err = toApiError(412, await problem(412, 'VERSION_MISMATCH').json(), () => true);
    const onMove = vi.fn(async () => Promise.reject(err));
    const { result } = renderHook(() => useOptimisticTopics(topics, onMove));
    await act(() => result.current.move(topics[1]!, { parent_topic_id: null, sort_order: 1 }));
    await waitFor(() => expect(result.current.error).toBe(err));
    expect(order(result.current.local)).toEqual(['a', 'b']);
  });

  it('If-Match comes from the row_version of the list item', () => {
    expect(ifMatchOf(5)).toEqual({ 'If-Match': 'W/"5"' });
    expect(ifMatchOf(undefined)).toEqual({});
  });
});
