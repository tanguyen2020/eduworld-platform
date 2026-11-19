// ifMatch (API_DESIGN §7, F050-AC-12): remember the ETag of the last GET of a resource (in memory) and send it
// as If-Match on operations that need it. Action paths `/x/{id}:publish` map to the resource `/x/{id}`.
export const IF_MATCH_HEADER = 'If-Match';

export function resourceKey(path: string): string {
  return path.replace(/:[A-Za-z][\w-]*$/, '');
}

export class EtagCache {
  private readonly etags = new Map<string, string>();

  remember(path: string, etag: string | null): void {
    if (etag) this.etags.set(resourceKey(path), etag);
  }

  get(path: string): string | undefined {
    return this.etags.get(resourceKey(path));
  }

  forget(path: string): void {
    this.etags.delete(resourceKey(path));
  }
}

export function applyIfMatch(headers: Headers, required: boolean, path: string, cache: EtagCache): void {
  if (!required || headers.has(IF_MATCH_HEADER)) return;
  const etag = cache.get(path);
  if (etag) headers.set(IF_MATCH_HEADER, etag);
}
