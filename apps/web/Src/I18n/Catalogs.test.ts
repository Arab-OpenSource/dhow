import { existsSync, readFileSync } from 'node:fs';
import { dirname, join, normalize } from 'node:path';

import { describe, expect, it } from 'vitest';

import { Ar } from '@/I18n/Locales/Ar';
import { BootAr } from '@/I18n/Locales/Boot/Ar';
import { BootEn } from '@/I18n/Locales/Boot/En';
import { En } from '@/I18n/Locales/En';
import { WorkspaceAr } from '@/I18n/Locales/Workspace/Ar';
import { WorkspaceEn } from '@/I18n/Locales/Workspace/En';

const SRC = join(process.cwd(), 'Src');
const ENTRIES = ['Main.tsx', 'Modules/Session/Pages/LoginPage.tsx', 'Modules/Identity/Pages/SetupPage.tsx'];
const STATIC_IMPORT = /^\s*(?:import|export)\s+(?!type\b)(?:[^'";]*?\bfrom\s*)?['"]([^'"]+)['"]/gm;
const KEY_LITERAL = /['"`]([a-zA-Z][\w.]*)['"`]/g;
const KEY_TEMPLATE = /`([a-zA-Z][\w.]*\.)\$\{/g;

const resolve = (spec: string, from: string): string | null => {
  const base = spec.startsWith('@/') ? join(SRC, spec.slice(2)) : spec.startsWith('.') ? normalize(join(dirname(from), spec)) : null;
  if (base === null) return null;
  return ['.ts', '.tsx'].map((extension) => base + extension).find((file) => existsSync(file)) ?? null;
};

const staticGraph = (): string[] => {
  const seen = new Set<string>();
  const queue = ENTRIES.map((entry) => join(SRC, entry));
  while (queue.length > 0) {
    const file = queue.pop();
    if (file === undefined || seen.has(file)) continue;
    seen.add(file);
    for (const [, spec] of readFileSync(file, 'utf8').matchAll(STATIC_IMPORT)) {
      const target = spec === undefined ? null : resolve(spec, file);
      if (target !== null && !target.includes('/I18n/Locales/')) queue.push(target);
    }
  }
  return [...seen];
};

const usedKeys = (files: readonly string[]): string[] => {
  const keys = new Set<string>();
  for (const file of files) {
    const source = readFileSync(file, 'utf8');
    for (const [, literal] of source.matchAll(KEY_LITERAL)) {
      if (literal !== undefined && Object.hasOwn(Ar, literal)) keys.add(literal);
    }
    for (const [, prefix] of source.matchAll(KEY_TEMPLATE)) {
      for (const key of Object.keys(Ar)) if (prefix !== undefined && key.startsWith(prefix)) keys.add(key);
    }
  }
  return [...keys].sort();
};

describe('the boot and workspace catalogs', () => {
  it('split the full catalog without sharing a key, in both languages', () => {
    const boot = Object.keys(BootAr);
    const workspace = Object.keys(WorkspaceAr);
    expect(boot.filter((key) => workspace.includes(key))).toEqual([]);
    expect([...boot, ...workspace].sort()).toEqual(Object.keys(Ar).sort());
    expect(Object.keys(BootEn).sort()).toEqual([...boot].sort());
    expect(Object.keys(WorkspaceEn).sort()).toEqual([...workspace].sort());
    expect(Object.keys(En).sort()).toEqual(Object.keys(Ar).sort());
  });

  it('keep every message the sign-in and setup pages need in the boot catalog, so they never wait for the shell', () => {
    const graph = staticGraph();
    expect(graph.some((file) => file.endsWith('/App/Layout/AppShell.tsx'))).toBe(false);
    expect(graph.some((file) => file.endsWith('/Modules/Session/Components/LoginForm.tsx'))).toBe(true);
    const missing = usedKeys(graph).filter((key) => !Object.hasOwn(BootAr, key));
    expect(missing).toEqual([]);
  });
});
