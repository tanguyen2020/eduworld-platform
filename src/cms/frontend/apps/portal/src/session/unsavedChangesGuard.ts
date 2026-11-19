// unsavedChangesGuard (F051 T040, F051-AC-20): editors of later features register a "dirty" probe; switching tenant
// (or signing out) asks for confirmation while one of them reports unsaved changes. Registry only — no UI here.

type DirtyProbe = () => boolean;

const probes = new Map<string, DirtyProbe>();

/** Registers a probe; returns the unregister function (call it on unmount). */
export function registerUnsavedChanges(id: string, isDirty: DirtyProbe): () => void {
  probes.set(id, isDirty);
  return () => {
    if (probes.get(id) === isDirty) probes.delete(id);
  };
}

export function hasUnsavedChanges(): boolean {
  for (const probe of probes.values()) {
    try {
      if (probe()) return true;
    } catch {
      // a broken probe never blocks the user
    }
  }
  return false;
}

export function clearUnsavedChangesForTest(): void {
  probes.clear();
}
