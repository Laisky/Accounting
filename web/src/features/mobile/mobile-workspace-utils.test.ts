import { describe, expect, it } from 'vitest';
import { meSectionFromPath, mobileTabFromPath } from './mobile-workspace-utils';

describe('mobile-workspace-utils', () => {
  it('keeps Me subpages on the Me tab', () => {
    expect(mobileTabFromPath('/me/profile')).toBe('me');
    expect(mobileTabFromPath('/me/security')).toBe('me');
  });

  it('parses canonical Me subpage routes', () => {
    expect(meSectionFromPath('/me')).toBe('index');
    expect(meSectionFromPath('/me/profile')).toBe('profile');
    expect(meSectionFromPath('/me/security')).toBe('security');
    expect(meSectionFromPath('/me/unknown')).toBeNull();
  });
});
