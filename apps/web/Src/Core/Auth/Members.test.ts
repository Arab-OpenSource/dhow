import { describe, expect, it } from 'vitest';

import type { IPage } from '@/Core/Api/Client';
import { coveringPage, type TMember } from '@/Core/Auth/useMembers';

const member = (index: number): TMember => ({
  UserId: `0198a000-0000-7000-8000-${String(index).padStart(12, '0')}`,
  Name: `Member ${String(index)}`,
  Email: `m${String(index)}@future.dev`,
  Role: 'developer',
  EffectiveRole: 'developer',
  JoinedAt: '2026-10-01T08:00:00Z',
});

const cachedPage = (page: number, limit: number, total: number): IPage<TMember> => {
  const start = (page - 1) * limit;
  const count = Math.max(0, Math.min(limit, total - start));
  return { Items: Array.from({ length: count }, (_, index) => member(start + index)), Meta: { Page: page, Limit: limit, Total: total } };
};

describe('reusing a cached page of members', () => {
  it.each([
    ['a small list for the full lookup', cachedPage(1, 25, 3), 1, 100, [0, 3]],
    ['the first page out of the full lookup', cachedPage(1, 100, 60), 1, 25, [0, 25]],
    ['a later page out of the full lookup', cachedPage(1, 100, 60), 3, 25, [50, 60]],
    ['an empty organization', cachedPage(1, 25, 0), 1, 100, [0, 0]],
  ] as const)('serves %s', (_, cached, page, limit, [from, to]) => {
    const served = coveringPage(cached, page, limit);
    expect(served?.Items.map((item) => item.UserId)).toEqual(cached.Items.slice(from, to).map((item) => item.UserId));
    expect(served?.Meta).toEqual({ Page: page, Limit: limit, Total: cached.Meta.Total });
  });

  it.each([
    ['a first page that holds only part of a bigger list', cachedPage(1, 25, 40), 1, 100],
    ['a page that starts after the one asked for', cachedPage(2, 25, 60), 1, 25],
    ['a page past the end of the list', cachedPage(1, 100, 30), 3, 25],
  ] as const)('asks the server for %s', (_, cached, page, limit) => {
    expect(coveringPage(cached, page, limit)).toBeUndefined();
  });
});
