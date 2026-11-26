// Hooks of the "Thử thách khối / giải trường" screen (F108 T027; F108-AC-14…AC-16): the only code of the screen that
// calls the network (AGENTS R1), one hook per F107 operation — listChallengeTemplates, listChallenges, createChallenge,
// getChallenge, cancelChallenge, getTournament, publishTournament, cancelTournament, registerTournamentEntry,
// withdrawTournamentEntry, confirmTournamentEntries, generateTournamentBracket, getTournamentBracket,
// confirmBracketNode, overrideBracketNode — plus the names of the students of one grade of the school
// (listTenantGradeStudents) for the organiser view of entries and bracket. Every call carries the X-Tenant-Id of the
// workspace C scope (a System Admin's picked tenant); query keys carry that tenant, so a tenant switch never shows
// another tenant's rows. Writes with a required Idempotency-Key send one key per user intent (useIdempotentMutation).
import { useMutation, useQueries, useQuery } from '@tanstack/react-query';
import { fetchAllPages, useAllPages } from '../../../../shared/api/fetchAllPages';
import { useIdempotentMutation } from '../../../../shared/api/useIdempotentMutation';
import type { components } from '../../../../shared/api/schema.gen';
import { LMS_PAGE_SIZE, lmsKey, useDataList, useLmsContext, type DataPage } from '../../lms/api/lmsCommon';
import type { Challenge, ChallengeCreate, ChallengeStatus, ChallengeTemplate } from '../model/challengeForm';

type S = components['schemas'];
export type Tournament = S['Tournament'];
export type Bracket = S['Bracket'];
export type BracketNode = S['BracketNode'];
export type TournamentEntry = S['TournamentEntry'];
export type TournamentEntryResult = S['TournamentEntryResult'];
export type GradeStudent = S['GradeStudent'];

export interface ChallengeFilters {
  gradeLevel?: number;
  status?: ChallengeStatus;
}

/** Templates assigned to the tenant (listChallengeTemplates) — the whole list for the wizard dropdown (every cursor page). */
export function useChallengeTemplates() {
  const { api, scope } = useLmsContext();
  return useAllPages<ChallengeTemplate>(
    lmsKey('challenge-templates', scope),
    (query) => api.call(api.client.GET('/lms/catalog/challenge-templates', { params: { query }, headers: scope.headers } as never)),
    { enabled: scope.ready },
  );
}

/** Challenges of the scope (listChallenges): cursor table, filtered by grade and status. */
export function useChallenges(f: ChallengeFilters = {}) {
  const { api, scope } = useLmsContext();
  return useDataList<Challenge>(
    lmsKey('challenges', scope, f.gradeLevel ?? '', f.status ?? ''),
    (cursor) =>
      api.call<DataPage<Challenge>>(
        api.client.GET('/lms/challenges', {
          params: { query: { limit: LMS_PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(f.gradeLevel ? { grade_level: f.gradeLevel } : {}), ...(f.status ? { status: f.status } : {}) } },
          headers: scope.headers,
        } as never),
      ),
    scope.ready,
  );
}

export function useChallenge(id: string | undefined) {
  const { api, scope } = useLmsContext();
  return useQuery({
    queryKey: lmsKey('challenge', scope, id),
    queryFn: () => api.call<Challenge>(api.client.GET('/lms/challenges/{challenge_id}', { params: { path: { challenge_id: id! } }, headers: scope.headers } as never)),
    enabled: scope.ready && !!id,
  });
}

/** createChallenge — one Idempotency-Key per intent: a double click or a retry with the same data creates one challenge. */
export function useCreateChallenge() {
  const { api, scope, invalidate } = useLmsContext();
  return useIdempotentMutation<Challenge, ChallengeCreate>(
    (body, intent) => api.call<Challenge>(api.client.POST('/lms/challenges', { body, headers: { ...scope.headers, ...intent } } as never)),
    { onSuccess: () => void invalidate('challenges') },
  );
}

/** Refreshes the challenge, its tournament, entries and bracket after a write. */
function useRefresh() {
  const { invalidate } = useLmsContext();
  return () => {
    for (const k of ['challenge', 'challenges', 'tournament', 'bracket']) void invalidate(k);
  };
}

export function useCancelChallenge(id: string) {
  const { api, scope } = useLmsContext();
  const refresh = useRefresh();
  return useIdempotentMutation<Challenge, { reason: string }>(
    (body, intent) => api.call<Challenge>(api.client.POST('/lms/challenges/{challenge_id}:cancel', { params: { path: { challenge_id: id } }, body, headers: { ...scope.headers, ...intent } } as never)),
    { onSuccess: refresh },
  );
}

export function useTournament(id: string | undefined) {
  const { api, scope } = useLmsContext();
  return useQuery({
    queryKey: lmsKey('tournament', scope, id),
    queryFn: () => api.call<Tournament>(api.client.GET('/competition/tournaments/{tournament_id}', { params: { path: { tournament_id: id! } }, headers: scope.headers } as never)),
    enabled: scope.ready && !!id,
  });
}

/** Entries (registrations, check-in, final rank), nodes and group standings of a tournament (getTournamentBracket). */
export function useTournamentBracket(id: string | undefined) {
  const { api, scope } = useLmsContext();
  return useQuery({
    queryKey: lmsKey('bracket', scope, id),
    queryFn: () => api.call<Bracket>(api.client.GET('/competition/tournaments/{tournament_id}/bracket', { params: { path: { tournament_id: id! } }, headers: scope.headers } as never)),
    enabled: scope.ready && !!id,
    retry: false,
  });
}

type TournamentOp = 'publish' | 'cancel' | 'generate-bracket';

/** publishTournament / cancelTournament (reason) / generateTournamentBracket — Idempotency-Key per intent. */
export function useTournamentAction(id: string, op: TournamentOp) {
  const { api, scope } = useLmsContext();
  const refresh = useRefresh();
  return useIdempotentMutation<Tournament | Bracket, { reason?: string }>(
    (v, intent) =>
      api.call<Tournament | Bracket>(
        api.client.POST(`/competition/tournaments/{tournament_id}:${op}` as '/competition/tournaments/{tournament_id}:publish', {
          params: { path: { tournament_id: id } },
          ...(op === 'cancel' ? { body: { reason: v.reason ?? '' } } : op === 'generate-bracket' ? { body: {} } : {}),
          headers: { ...scope.headers, ...intent },
        } as never),
      ),
    { onSuccess: refresh },
  );
}

/** registerTournamentEntry by the organiser with an empty body = the whole audience of the challenge (grade ± students). */
export function useRegisterAudience(id: string) {
  const { api, scope } = useLmsContext();
  const refresh = useRefresh();
  return useIdempotentMutation<TournamentEntryResult, Record<string, never>>(
    (body, intent) => api.call<TournamentEntryResult>(api.client.POST('/competition/tournaments/{tournament_id}/entries', { params: { path: { tournament_id: id } }, body, headers: { ...scope.headers, ...intent } } as never)),
    { onSuccess: refresh },
  );
}

/** confirmTournamentEntries with an empty body (every registered entry). */
export function useConfirmEntries(id: string) {
  const { api, scope } = useLmsContext();
  const refresh = useRefresh();
  return useIdempotentMutation<TournamentEntryResult, Record<string, never>>(
    (body, intent) => api.call<TournamentEntryResult>(api.client.POST('/competition/tournaments/{tournament_id}/entries:confirm', { params: { path: { tournament_id: id } }, body, headers: { ...scope.headers, ...intent } } as never)),
    { onSuccess: refresh },
  );
}

export function useWithdrawEntry(id: string) {
  const { api, scope } = useLmsContext();
  const refresh = useRefresh();
  return useMutation({
    mutationFn: (entryId: string) =>
      api.call<void>(api.client.DELETE('/competition/tournaments/{tournament_id}/entries/{entry_id}', { params: { path: { tournament_id: id, entry_id: entryId } }, headers: scope.headers } as never)),
    onSuccess: refresh,
  });
}

export function useConfirmNode(id: string) {
  const { api, scope } = useLmsContext();
  const refresh = useRefresh();
  return useMutation({
    mutationFn: (nodeId: string) =>
      api.call<BracketNode>(api.client.POST('/competition/tournaments/{tournament_id}/nodes/{node_id}:confirm', { params: { path: { tournament_id: id, node_id: nodeId } }, headers: scope.headers } as never)),
    onSuccess: refresh,
  });
}

export interface OverrideVars {
  nodeId: string;
  winner_entry_id: string;
  reason: string;
}

/** overrideBracketNode (winner + mandatory reason) — Idempotency-Key per intent. */
export function useOverrideNode(id: string) {
  const { api, scope } = useLmsContext();
  const refresh = useRefresh();
  return useIdempotentMutation<BracketNode, OverrideVars>(
    ({ nodeId, ...body }, intent) =>
      api.call<BracketNode>(
        api.client.POST('/competition/tournaments/{tournament_id}/nodes/{node_id}:override', { params: { path: { tournament_id: id, node_id: nodeId } }, body, headers: { ...scope.headers, ...intent } } as never),
      ),
    { onSuccess: refresh },
  );
}

/**
 * Real names of the students of some grades of the school (listTenantGradeStudents, every page of each grade) for the
 * organiser view: TournamentPlayer only carries the nickname (R16 — the player view); the portal staff of the school
 * see real names. The caller passes the grade of the challenge, plus the other grades when students were added.
 */
export function useSchoolStudentNames(schoolId: string | null | undefined, grades: readonly number[]): Map<string, string> {
  const { api, scope } = useLmsContext();
  const results = useQueries({
    queries: grades.map((grade) => ({
      queryKey: [...lmsKey('grade-student-names', scope, schoolId, grade), 'all-pages'],
      queryFn: () =>
        fetchAllPages<GradeStudent>((query) =>
          api.call(
            api.client.GET('/tenant/schools/{school_id}/grades/{grade_level}/students', { params: { path: { school_id: schoolId!, grade_level: grade }, query }, headers: scope.headers } as never),
          ),
        ),
      enabled: scope.ready && !!schoolId,
      staleTime: 60_000,
    })),
  });
  return new Map(results.flatMap((r) => r.data?.items ?? []).map((s) => [s.student_id, s.display_name]));
}
