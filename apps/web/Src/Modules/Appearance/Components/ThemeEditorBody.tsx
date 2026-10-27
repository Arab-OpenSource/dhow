import { zodResolver } from '@hookform/resolvers/zod';
import { useEffectEvent, useLayoutEffect, useState } from 'react';
import { useForm, type UseFormRegisterReturn } from 'react-hook-form';

import { deriveBackgrounds } from '@/Core/Theme/ThemeColor';
import { EDITOR_TOKEN_GROUPS, SYSTEM_THEMES, type TEditorGroup, type TThemeColors } from '@/Core/Theme/ThemeTokens';
import { THEME_NAME_MAX } from '@/Core/Theme/ThemeValidation';
import { useThemeStore, type IThemeDraft } from '@/Core/Theme/useThemeStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { Field } from '@/Core/Ui/Field';
import type { TIconName } from '@/Core/Ui/Icons';
import { Input } from '@/Core/Ui/Input';
import { Rows } from '@/Core/Ui/Row';
import { isMessageKey } from '@/I18n/Translate';
import { useT } from '@/I18n/useT';
import { ColorRow } from '@/Modules/Appearance/Components/ColorRow';
import { ContrastCard } from '@/Modules/Appearance/Components/ContrastCard';
import { useLastDraft } from '@/Modules/Appearance/Hooks/useLastDraft';
import { THEME_FORM_ID } from '@/Modules/Appearance/Types/ThemeEditorIds';
import { themeFormSchema, type TThemeForm } from '@/Modules/Appearance/Types/ThemeForm';

const GROUP_ICONS: Record<TEditorGroup, TIconName> = {
  bg: 'layers',
  text: 'file',
  aurora: 'shield',
  status: 'info',
};

export interface IThemeEditorBodyProps {
  sourceId?: string;
  onDone: () => void;
}

interface IStartCardProps {
  nameError: string | undefined;
  nameField: UseFormRegisterReturn;
  base: string;
  onBase: (value: string) => void;
}

const StartCard = ({ nameError, nameField, base, onBase }: IStartCardProps) => {
  const t = useT();
  const SetDraftColors = useThemeStore((state) => state.SetDraftColors);

  return (
    <Card title={t('themeEditor.basics')} icon="user" padded>
      <div className="grid gap-4">
        <Field label={t('themeEditor.name')} error={nameError}>
          <Input maxLength={THEME_NAME_MAX} autoComplete="off" {...nameField} />
        </Field>
        <div className="grid gap-2">
          <span className="text-body-sm text-secondary">{t('themeEditor.startFrom')}</span>
          <div className="flex flex-wrap gap-2">
            {SYSTEM_THEMES.map((theme) => (
              <Button
                key={theme.Id}
                size="sm"
                onClick={() => {
                  SetDraftColors(theme.Colors);
                  onBase(theme.Colors.surface);
                }}
              >
                {t(`theme.system.${theme.Id}`)}
              </Button>
            ))}
          </div>
        </div>
        <div className="flex items-center gap-3">
          <span className="min-w-0 flex-1 text-body-sm text-secondary">{t('themeEditor.generate')}</span>
          <input
            type="color"
            value={base}
            aria-label={t('themeEditor.generate.label')}
            onChange={(event) => {
              onBase(event.target.value);
              SetDraftColors(deriveBackgrounds(event.target.value));
            }}
            className="size-btn-sm cursor-pointer rounded-input border-0 bg-transparent p-0"
          />
        </div>
      </div>
    </Card>
  );
};

const GroupCards = ({ colors }: { colors: TThemeColors }) => {
  const t = useT();
  const SetDraftColor = useThemeStore((state) => state.SetDraftColor);

  return EDITOR_TOKEN_GROUPS.map((group) => (
    <Card key={group.Group} title={t(`theme.group.${group.Group}`)} icon={GROUP_ICONS[group.Group]}>
      <Rows>
        {group.Tokens.map((token) => (
          <ColorRow
            key={token}
            label={t(`theme.token.${token}`)}
            value={colors[token]}
            onChange={(value) => {
              SetDraftColor(token, value);
            }}
          />
        ))}
      </Rows>
    </Card>
  ));
};

const EditorForm = ({ draft, onDone }: { draft: IThemeDraft; onDone: () => void }) => {
  const t = useT();
  const SetDraftName = useThemeStore((state) => state.SetDraftName);
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
      className="grid gap-3.5"
    >
      <ContrastCard colors={draft.Colors} />
      <StartCard
        nameError={nameError}
        nameField={form.register('Name', {
          onChange: (event: { target: { value: string } }) => {
            SetDraftName(event.target.value);
          },
        })}
        base={Base}
        onBase={setBase}
      />
      <GroupCards colors={draft.Colors} />
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
