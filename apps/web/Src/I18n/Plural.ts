import type { TLocale } from '@/I18n/Locale';
import type { TMessageParams } from '@/I18n/Messages';

const RULES: Record<TLocale, Intl.PluralRules> = { ar: new Intl.PluralRules('ar'), en: new Intl.PluralRules('en') };

const HEAD = /\{(\w+),\s*plural,/g;
const FORM = /\s*(zero|one|two|few|many|other|=\d+)\s*\{/y;

const closingBrace = (text: string, open: number): number => {
  let depth = 0;
  for (let index = open; index < text.length; index += 1) {
    if (text[index] === '{') depth += 1;
    if (text[index] === '}') {
      depth -= 1;
      if (depth === 0) return index;
    }
  }
  return -1;
};

const formsOf = (body: string): Map<string, string> => {
  const forms = new Map<string, string>();
  let cursor = 0;
  for (;;) {
    FORM.lastIndex = cursor;
    const match = FORM.exec(body);
    if (match?.[1] === undefined) return forms;
    const open = FORM.lastIndex - 1;
    const close = closingBrace(body, open);
    if (close < 0) return forms;
    forms.set(match[1], body.slice(open + 1, close));
    cursor = close + 1;
  }
};

export const pluralCategory = (locale: TLocale, count: number): Intl.LDMLPluralRule => RULES[locale].select(count);

export const selectPlurals = (locale: TLocale, template: string, params?: TMessageParams): string => {
  let out = '';
  let cursor = 0;
  for (const head of template.matchAll(HEAD)) {
    const name = head[1] ?? '';
    if (head.index < cursor) continue;
    const close = closingBrace(template, head.index);
    if (close < 0) break;
    const forms = formsOf(template.slice(head.index + head[0].length, close));
    const value = Number(params?.[name]);
    const exact = Number.isFinite(value) ? forms.get(`=${String(value)}`) : undefined;
    const category = Number.isFinite(value) ? pluralCategory(locale, value) : 'other';
    out += template.slice(cursor, head.index) + (exact ?? forms.get(category) ?? forms.get('other') ?? '');
    cursor = close + 1;
  }
  return out + template.slice(cursor);
};
