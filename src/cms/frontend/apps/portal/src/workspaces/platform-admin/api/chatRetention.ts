// Gameplay chat retention hooks (F057 T037, F057-AC-16, ADR-0033): GET (ETag, read-only `minimum_days`) and PUT with
// If-Match and a mandatory reason; `422 RETENTION_BELOW_MINIMUM` is shown on the field by its code.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { usePlatformApi, type Schemas } from './common';

export type ChatRetentionSettings = Schemas['ChatRetentionSettings'];
export type ChatRetentionInput = Omit<ChatRetentionSettings, 'minimum_days'>;

const KEY = ['platform', 'settings', 'chat-retention'];

export function useChatRetention() {
  const api = usePlatformApi();
  return useQuery({ queryKey: KEY, queryFn: () => api.call(api.client.GET('/admin/settings/chat-retention', {})) });
}

export function usePutChatRetention() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ChatRetentionInput) => api.call(api.client.PUT('/admin/settings/chat-retention', { body: body as ChatRetentionSettings })),
    onSuccess: (data) => qc.setQueryData(KEY, data),
  });
}
