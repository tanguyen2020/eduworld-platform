// @vitest-environment node
// F051-AC-07/28/29: only same-origin paths are a return target after sign-in (no open redirect, no auth page).
import { describe, expect, it } from 'vitest';
import { loginPath, returnTarget, safeReturnUrl } from './returnUrl';

describe('returnUrl', () => {
  it.each(['/school-lms/students?page=2', '/platform-admin/tenants#x'])('%s is kept', (u) => {
    expect(safeReturnUrl(u)).toBe(u);
  });

  it.each(['https://evil.example/x', '//evil.example', '/\\evil', 'school-lms', '/login', '/reset-password?token=x', '/a\u0000b', ''])('%s is refused', (u) => {
    expect(safeReturnUrl(u)).toBeNull();
  });

  it('builds /login?returnUrl= and reads it back', () => {
    expect(loginPath('/school-lms/students')).toBe('/login?returnUrl=%2Fschool-lms%2Fstudents');
    expect(loginPath('/')).toBe('/login');
    expect(returnTarget('?returnUrl=%2Fschool-lms%2Fstudents')).toBe('/school-lms/students');
    expect(returnTarget('?returnUrl=https%3A%2F%2Fevil')).toBe('/');
  });
});
