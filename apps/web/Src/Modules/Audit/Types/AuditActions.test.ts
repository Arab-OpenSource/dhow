import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { describe, expect, it } from 'vitest';

import { Ar } from '@/I18n/Locales/Ar';
import { En } from '@/I18n/Locales/En';
import { AuditAr } from '@/I18n/Locales/Workspace/AuditAr';
import { AuditEn } from '@/I18n/Locales/Workspace/AuditEn';
import { actionGroup, AUDIT_ACTIONS } from '@/Modules/Audit/Types/AuditActions';

const REGISTRY = join(process.cwd(), '..', 'server', 'internal', 'Core', 'EventBus', 'Events.Registry.go');

const serverActions = (): string[] => {
  const source = readFileSync(REGISTRY, 'utf8');
  const constants = new Map<string, string>();
  for (const [, name, value] of source.matchAll(/^\s*(\w+)\s*=\s*"([^"]+)"/gm)) {
    if (name !== undefined && value !== undefined) constants.set(name, value);
  }
  const list = /func AuditedEvents\(\) \[\]string \{\s*return \[\]string\{([^}]*)\}/.exec(source)?.[1] ?? '';
  return [...list.matchAll(/(\w+),/g)].map(([, name]) => constants.get(name ?? '') ?? `missing constant ${name ?? ''}`);
};

describe('the audit action catalog', () => {
  it('reads the list of audited events from the server', () => {
    const actions = serverActions();
    expect(actions.length).toBeGreaterThan(40);
    expect(actions).toEqual(expect.arrayContaining(['auth.login', 'job.created', 'runner.registered']));
  });

  it('has an icon and a label in both languages for every action the server can write', () => {
    const missing = serverActions().filter(
      (action) => !(action in AUDIT_ACTIONS) || !(`audit.action.${action}` in AuditAr) || !(`audit.action.${action}` in AuditEn),
    );
    expect(missing).toEqual([]);
  });

  it('keeps every action label in the audit catalogs, so no other catalog overrides one', () => {
    for (const [full, audit] of [
      [Ar, AuditAr],
      [En, AuditEn],
    ] as const) {
      const labels = Object.entries(full).filter(([key]) => key.startsWith('audit.action.'));
      expect(labels).toEqual(Object.entries(audit).filter(([key]) => key.startsWith('audit.action.')));
    }
  });

  it('has no entry the server never writes', () => {
    const actions = new Set(serverActions());
    expect(Object.keys(AUDIT_ACTIONS).filter((action) => !actions.has(action))).toEqual([]);
  });

  it('puts every action in an area that has a readable fallback name', () => {
    expect(actionGroup('runner.something_new')).toBe('runner');
    expect(actionGroup('job.retried')).toBe('job');
    expect(actionGroup('billing.invoice.paid')).toBe('other');
    expect(actionGroup('')).toBe('other');
    for (const group of ['auth', 'identity', 'runner', 'job', 'system', 'other']) {
      expect(AuditAr).toHaveProperty([`audit.group.${group}`]);
      expect(AuditEn).toHaveProperty([`audit.group.${group}`]);
    }
  });
});
