// Console API hooks of workspace A (F079 T021, T026, T034, T045): client manifest of the reference pickers
// (getGameClientManifest, AC-07), server-side simulation (simulateGameConfig, AC-10/AC-15 — the portal never computes
// progress, AGENTS R1), dashboard counts (getGameConfigCounts), preview (createGamePreviewSession → token kept in memory
// only, then getGamePreviewSnapshot with X-Preview-Token, AC-17) and game media upload (presignGameMediaUpload,
// getGameMediaAsset).
import { useMutation, useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import type { ConfigType } from '../registry/configTypes';

export type ClientManifest = components['schemas']['GameClientManifest'];
export type Simulation = components['schemas']['GameConfigSimulation'];
export type ConfigCounts = components['schemas']['GameConfigCounts'];
export type MediaAsset = components['schemas']['MediaAsset'];
export type MediaPresignRequest = components['schemas']['MediaPresignRequest'];
export type MediaPresignResponse = components['schemas']['MediaPresignResponse'];

export interface PreviewSnapshot {
  config_type: string;
  config_id: string;
  version: number;
  status: string;
  title?: string;
  payload: Record<string, unknown>;
  expires_at: string;
}

export function useClientManifest() {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['game-ops', 'manifest', scopeKey],
    queryFn: () => api.call<ClientManifest>(api.client.GET('/game-admin/client-manifests/current', {})),
    staleTime: 5 * 60_000,
    retry: false,
  });
}

export function useConfigCounts() {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['game-ops', 'counts', scopeKey],
    queryFn: () => api.call<ConfigCounts>(api.client.GET('/game-admin/stats/config-counts', {})),
  });
}

export interface SimulationInput {
  configType: Extract<ConfigType, 'progression-curves' | 'season-tracks'>;
  configId: string;
  version: number;
  minutesPerDay?: number;
  daysPerWeek: number;
  weeks: number;
}

/** Runs the server simulation of a saved version (a read: no Idempotency-Key). */
export function useSimulation() {
  const { api } = usePortalRuntime();
  return useMutation({
    mutationFn: (v: SimulationInput) =>
      api.call<Simulation>(
        api.client.GET('/game-admin/{config_type}/{config_id}/versions/{version}:simulate', {
          params: {
            path: { config_type: v.configType, config_id: v.configId, version: v.version },
            query: { ...(v.minutesPerDay ? { minutes_per_day: v.minutesPerDay } : {}), days_per_week: v.daysPerWeek, weeks: v.weeks },
          },
        }),
      ),
  });
}

/** Preview of one version: session (Idempotency-Key) then the snapshot read with the token; the token is not stored. */
export function usePreview() {
  const { api } = usePortalRuntime();
  return useIdempotentMutation(async (v: { configType: ConfigType; configId: string; version: number }, intent) => {
    const session = await api.call<{ preview_token: string; expires_at: string }>(
      api.client.POST('/game-admin/preview/sessions', { body: { config_type: v.configType, config_id: v.configId, version: v.version }, headers: intent as never }),
    );
    return api.call<PreviewSnapshot>(
      api.client.GET('/game-admin/preview/snapshot', {
        params: { header: { 'X-Preview-Token': session.preview_token }, query: { config_type: v.configType, config_id: v.configId, version: v.version } },
      }),
    );
  });
}

export function useMediaPresign() {
  const { api } = usePortalRuntime();
  return useIdempotentMutation((body: MediaPresignRequest, intent) =>
    api.call<MediaPresignResponse>(api.client.POST('/game-media:presign-upload', { body, headers: intent as never })),
  );
}

export function useMediaAsset(assetId: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['game-ops', 'media', scopeKey, assetId],
    queryFn: () => api.call<MediaAsset>(api.client.GET('/game-media/{asset_id}', { params: { path: { asset_id: assetId! } } })),
    enabled: !!assetId,
    refetchInterval: (q) => (q.state.data && (q.state.data as MediaAsset).status !== 'ready' && (q.state.data as MediaAsset).status !== 'rejected' ? 5_000 : false),
  });
}
