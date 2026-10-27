import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { Ar } from '@/I18n/Locales/Ar';
import { En } from '@/I18n/Locales/En';
import { I18nProvider } from '@/I18n/I18nProvider';
import { STORAGE_KEY } from '@/I18n/Locale';
import { translate } from '@/I18n/Translate';
import { useI18n } from '@/I18n/useI18n';

const placeholders = (text: string): string[] => [...text.matchAll(/\{(\w+)\}/g)].map((match) => match[1] ?? '').sort();

describe('locale catalogs', () => {
  it('have exactly the same keys in Arabic and English', () => {
    expect(Object.keys(En).sort()).toEqual(Object.keys(Ar).sort());
  });

  it('use the same placeholders in both languages and never ship an empty message', () => {
    for (const [key, arabic] of Object.entries(Ar)) {
      const english = En[key as keyof typeof En];
      expect(placeholders(english), key).toEqual(placeholders(arabic));
      expect(arabic.trim(), key).not.toBe('');
      expect(english.trim(), key).not.toBe('');
    }
  });

  it('write the Arabic catalog in Arabic script', () => {
    const arabicLetters = /[؀-ۿ]/;
    const untranslated = Object.entries(Ar)
      .filter(([key, text]) => key !== 'language.en' && !arabicLetters.test(text))
      .map(([key]) => key);
    expect(untranslated).toEqual([]);
  });
});

describe('translate', () => {
  it('fills placeholders and leaves unknown ones visible', () => {
    expect(translate('en', 'shell.language.switch', { language: 'Arabic' })).toBe('Switch the language to Arabic');
    expect(translate('en', 'shell.language.switch')).toBe('Switch the language to {language}');
  });
});

const Probe = () => {
  const { Locale, SetLocale, t } = useI18n();
  return (
    <>
      <p>{t('workspace.title')}</p>
      <output>{Locale}</output>
      <button
        type="button"
        onClick={() => {
          SetLocale(Locale === 'ar' ? 'en' : 'ar');
        }}
      >
        flip
      </button>
    </>
  );
};

describe('I18nProvider', () => {
  it('starts in Arabic, right to left', () => {
    render(
      <I18nProvider>
        <Probe />
      </I18nProvider>,
    );
    expect(screen.getByText(Ar['workspace.title'])).toBeInTheDocument();
    expect(document.documentElement.lang).toBe('ar');
    expect(document.documentElement.dir).toBe('rtl');
    expect(document.title).toBe(Ar['app.title']);
  });

  it('flips lang, dir and the title together and remembers the choice', async () => {
    render(
      <I18nProvider>
        <Probe />
      </I18nProvider>,
    );
    await userEvent.click(screen.getByRole('button', { name: 'flip' }));
    expect(screen.getByText(En['workspace.title'])).toBeInTheDocument();
    expect(document.documentElement.lang).toBe('en');
    expect(document.documentElement.dir).toBe('ltr');
    expect(document.title).toBe(En['app.title']);
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('en');
    await userEvent.click(screen.getByRole('button', { name: 'flip' }));
    expect(document.documentElement.dir).toBe('rtl');
  });

  it('restores the saved language on the next visit', () => {
    window.localStorage.setItem(STORAGE_KEY, 'en');
    render(
      <I18nProvider>
        <Probe />
      </I18nProvider>,
    );
    expect(screen.getByRole('status')).toHaveTextContent('en');
    expect(document.documentElement.dir).toBe('ltr');
  });

  it('falls back to Arabic for an unsupported saved language', () => {
    window.localStorage.setItem(STORAGE_KEY, 'fr');
    render(
      <I18nProvider>
        <Probe />
      </I18nProvider>,
    );
    expect(screen.getByRole('status')).toHaveTextContent('ar');
  });

  it('keeps working when storage is blocked', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new DOMException('blocked', 'SecurityError');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('blocked', 'SecurityError');
    });
    render(
      <I18nProvider>
        <Probe />
      </I18nProvider>,
    );
    expect(screen.getByRole('status')).toHaveTextContent('ar');
    await userEvent.click(screen.getByRole('button', { name: 'flip' }));
    expect(screen.getByRole('status')).toHaveTextContent('en');
    expect(document.documentElement.dir).toBe('ltr');
  });
});
