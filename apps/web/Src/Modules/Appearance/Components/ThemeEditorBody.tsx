import { zodResolver } from '@hookform/resolvers/zod';
import { useEffectEvent, useLayoutEffect, useState } from 'react';
import { useForm } from 'react-hook-form';

import { deriveBackgrounds } from '@/Core/Theme/ThemeColor';
import { SYSTEM_THEMES, THEME_TOKEN_GROUPS } from '@/Core/Theme/ThemeTokens';
import { THEME_NAME_MAX } from '@/Core/Theme/ThemeValidation';
import { useThemeStore, type IThemeDraft } from '@/Core/Theme/useThemeStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { isMessageKey } from '@/I18n/Translate';
import { useT } from '@/I18n/useT';
import { ColorRow } from '@/Modules/Appearance/Components/ColorRow';
import { ContrastBadges } from '@/Modules/Appearance/Components/ContrastBadges';
import { useLastDraft } from '@/Modules/Appearance/Hooks/useLastDraft';
import { THEME_FORM_ID } from '@/Modules/Appearance/Types/ThemeEditorIds';
import { themeFormSchema, type TThemeForm } from '@/Modules/Appearance/Types/ThemeForm';

export interface IThemeEditorBodyProps {
  sourceId?: string;
  onDone: () => void;
}

const Heading = ({ children }: { children: string }) => (
  <h3 className="m-0 mt-1 text-caption font-normal text-tertiary">{children}</h3>
);

const EditorForm = ({ draft, onDone }: { draft: IThemeDraft; onDone: () => void }) => {
  const t = useT();
  const SetDraftName = useThemeStore((state) => state.SetDraftName);
  const SetDraftColor = useThemeStore((state) => state.SetDraftColor);
  const SetDraftColors = useThemeStore((state) => state.SetDraftColors);
  const SaveDraft = useThemeStore((state) => state.SaveDraft);
  const Push = useToastStore((state) => state.Push);
  const [Base, setBase] = useState(draft.Colors.surface);
  const form = useForm<TThemeForm>({
    resolver: zodResolver(themeFormSchema),
    defaultValues: { Name: draft.Name },
    mode: 'onChange',
  });
  const message = form.formState.errors.Name?.message;
  const nameError = message !== undefined && isMessageKey(message) ? t(message) : undefined;

  const Save = form.handleSubmit(() => {
    const saved = SaveDraft();
    if (!saved) return;
    Push({ Id: `theme-saved-${saved.Id}`, Accent: 'live', Title: t('themeEditor.saved', { name: saved.Name }) });
    onDone();
  });

  return (
    <form
      id={THEME_FORM_ID}
      noValidate
      onSubmit={(event) => {
        void Save(event);
      }}
      className="grid gap-3"
    >
      <label className="grid gap-1.5 text-caption text-secondary">
        {t('themeEditor.name')}
        <input
          type="text"
          maxLength={THEME_NAME_MAX}
          autoComplete="off"
          aria-invalid={nameError ? true : undefined}
          aria-describedby={nameError ? 'theme-name-error' : undefined}
          className="h-[42px] rounded-md border border-default bg-input px-3 text-body-sm text-primary aria-invalid:border-accent-coral"
          {...form.register('Name', {
            onChange: (event: { target: { value: string } }) => {
              SetDraftName(event.target.value);
            },
          })}
        />
      </label>
      {nameError ? (
        <p id="theme-name-error" role="alert" className="m-0 text-micro text-accent-coral">
          {nameError}
        </p>
      ) : null}
      <Heading>{t('themeEditor.startFrom')}</Heading>
      <div className="flex flex-wrap gap-1.5">
        {SYSTEM_THEMES.map((theme) => (
          <Button
            key={theme.Id}
            size="sm"
            onClick={() => {
              SetDraftColors(theme.Colors);
              setBase(theme.Colors.surface);
            }}
          >
            {t(`theme.system.${theme.Id}`)}
          </Button>
        ))}
      </div>
      <div className="flex items-center gap-2.5 rounded-md bg-row py-1.5 ps-3 pe-2 text-caption">
        <span className="min-w-0 flex-1">{t('themeEditor.generate')}</span>
        <input
          type="color"
          value={Base}
          aria-label={t('themeEditor.generate.label')}
          onChange={(event) => {
            setBase(event.target.value);
            SetDraftColors(deriveBackgrounds(event.target.value));
          }}
          className="size-[34px] cursor-pointer rounded-[10px] border-0 bg-transparent p-0"
        />
      </div>
      <ContrastBadges colors={draft.Colors} />
      {THEME_TOKEN_GROUPS.map((group) => (
        <div key={group.Group} className="grid gap-1.5">
          <Heading>{t(`theme.group.${group.Group}`)}</Heading>
          {group.Tokens.map((token) => (
            <ColorRow
              key={token}
              label={t(`theme.token.${token}`)}
              value={draft.Colors[token]}
              onChange={(value) => {
                SetDraftColor(token, value);
              }}
            />
          ))}
        </div>
      ))}
    </form>
  );
};

export const ThemeEditorBody = ({ sourceId, onDone }: IThemeEditorBodyProps) => {
  const t = useT();
  const BeginDraft = useThemeStore((state) => state.BeginDraft);
  const draft = useLastDraft();

  const begin = useEffectEvent(() => {
    BeginDraft({ SourceId: sourceId, DefaultName: t('themeEditor.defaultName') });
  });

  useLayoutEffect(() => {
    begin();
  }, [sourceId]);

  return draft ? <EditorForm draft={draft} onDone={onDone} /> : null;
};
