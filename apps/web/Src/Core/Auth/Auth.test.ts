import { describe, expect, it } from 'vitest';

import { canEditOrganization, canReadAudit, canReadMembers, ROLES, secondFactorCovers } from '@/Core/Auth/AuthTypes';
import { isLengthValid, PASSWORD_MAX_LENGTH, scorePassword } from '@/Core/Auth/PasswordStrength';
import { agoOf, formatClock, formatDateTime } from '@/Core/Format/TimeFormat';

describe('password strength', () => {
  it.each([
    ['', 0],
    ['short', 1],
    ['aaaaaaaaaaaaaaaa', 1],
    ['abcdefghijkl', 2],
    ['abcdefgh1234IJKL', 3],
    ['correct horse battery staple 42', 4],
    ['كلمة سر طويلة جدا للتجربة 2026', 4],
  ])('rates %j as %i', (password, score) => {
    expect(scorePassword(password)).toBe(score);
  });

  it('accepts only 12 to 128 characters as long enough', () => {
    expect(isLengthValid('a'.repeat(11))).toBe(false);
    expect(isLengthValid('a'.repeat(12))).toBe(true);
    expect(isLengthValid('a'.repeat(PASSWORD_MAX_LENGTH))).toBe(true);
    expect(isLengthValid('a'.repeat(PASSWORD_MAX_LENGTH + 1))).toBe(false);
  });
});

describe('what each role may open', () => {
  it('lets only the owner and admins read the audit log and rename the organization', () => {
    expect(ROLES.filter(canReadAudit)).toEqual(['owner', 'admin']);
    expect(ROLES.filter(canEditOrganization)).toEqual(['owner', 'admin']);
  });

  it('keeps the member list from viewers', () => {
    expect(ROLES.filter(canReadMembers)).toEqual(['owner', 'admin', 'maintainer', 'developer']);
    expect(canReadAudit(undefined)).toBe(false);
  });
});

describe('time formatting', () => {
  const NOW = Date.parse('2026-10-27T12:00:00Z');

  it('formats a countdown as minutes and seconds', () => {
    expect(formatClock(28)).toBe('0:28');
    expect(formatClock(75)).toBe('1:15');
    expect(formatClock(-3)).toBe('0:00');
  });

  it.each([
    ['2026-10-27T11:59:40Z', { Key: 'time.now' }],
    ['2026-10-27T11:59:00Z', { Key: 'time.minute.one' }],
    ['2026-10-27T11:15:00Z', { Key: 'time.minutes', Count: 45 }],
    ['2026-10-27T11:00:00Z', { Key: 'time.hour.one' }],
    ['2026-10-26T12:00:00Z', { Key: 'time.day.one' }],
    ['2026-10-17T12:00:00Z', { Key: 'time.days', Count: 10 }],
    ['2026-07-01T12:00:00Z', null],
    ['not a date', null],
  ])('describes %s as %j', (iso, expected) => {
    expect(agoOf(iso, NOW)).toEqual(expected);
  });

  it('formats an absolute time in both languages and returns nothing for garbage', () => {
    expect(formatDateTime('2026-10-27T12:00:00Z', 'en')).toContain('2026');
    expect(formatDateTime('2026-10-27T12:00:00Z', 'ar')).toMatch(/2026/);
    expect(formatDateTime('nope', 'en')).toBe('');
  });
});

describe('who the second-step rule covers', () => {
  it('matches the server: nobody when off, owners and admins for admins, every role for everyone', () => {
    expect(ROLES.filter((role) => secondFactorCovers('off', role))).toEqual([]);
    expect(ROLES.filter((role) => secondFactorCovers('admins', role))).toEqual(['owner', 'admin']);
    expect(ROLES.filter((role) => secondFactorCovers('everyone', role))).toEqual(ROLES);
  });
});
