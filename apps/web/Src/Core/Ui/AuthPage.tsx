import type { ReactNode } from 'react';

import { Button } from '@/Core/Ui/Button';
import { Mark } from '@/Core/Ui/Mark';
import { otherLocale } from '@/I18n/Locale';
import { useI18n } from '@/I18n/useI18n';

export interface IAuthPageProps {
  title: string;
  lead?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}

export const AuthPage = ({ title, lead, footer, children }: IAuthPageProps) => {
  const { Locale, SetLocale, t } = useI18n();
  const target = otherLocale(Locale);

  return (
    <div className="flex min-h-dvh flex-col bg-canvas">
      <header className="flex items-center gap-2.5 px-4 py-4 md:px-8">
        <Mark size={28} />
        <span className="text-title-section">{t('shell.brand')}</span>
        <Button
          className="ms-auto"
          aria-label={t('shell.language.switch', { language: t(`language.${target}`) })}
          onClick={() => {
            SetLocale(target);
          }}
        >
          <span lang={target}>{t(`language.${target}`)}</span>
        </Button>
      </header>
      <main id="main" tabIndex={-1} className="grid flex-1 content-start justify-items-center px-4 pt-4 pb-10 md:content-center md:pt-0">
        <div className="grid w-full max-w-[440px] gap-4">
          <section className="grid gap-5 rounded-lg border border-line bg-surface p-5 md:p-8">
            <div className="grid gap-1.5">
              <h1 className="m-0 text-title-page">{title}</h1>
              {lead ? <p className="m-0 text-body-sm text-secondary">{lead}</p> : null}
            </div>
            {children}
          </section>
          {footer ? <div className="text-center text-body-sm text-secondary">{footer}</div> : null}
        </div>
      </main>
    </div>
  );
};
