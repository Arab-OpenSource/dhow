import { useThemeStore } from '@/Core/Theme/useThemeStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import { useLastDraft } from '@/Modules/Appearance/Hooks/useLastDraft';
import { THEME_FORM_ID } from '@/Modules/Appearance/Types/ThemeEditorIds';

export interface IThemeEditorFooterProps {
  onCancel: () => void;
  onDone: () => void;
}

export const ThemeEditorFooter = ({ onCancel, onDone }: IThemeEditorFooterProps) => {
  const t = useT();
  const draft = useLastDraft();
  const ActiveId = useThemeStore((state) => state.ActiveId);
  const RemoveTheme = useThemeStore((state) => state.RemoveTheme);
  const Push = useToastStore((state) => state.Push);

  const Delete = (id: string) => {
    const wasActive = id === ActiveId;
    RemoveTheme(id);
    Push({
      Id: `theme-deleted-${id}`,
      Accent: 'live',
      Title: t(wasActive ? 'themeEditor.deletedActive' : 'themeEditor.deleted'),
    });
    onDone();
  };

  return (
    <>
      {draft?.Existing ? (
        <Button
          variant="danger"
          iconStart="trash"
          onClick={() => {
            Delete(draft.Id);
          }}
        >
          {t('themeEditor.delete')}
        </Button>
      ) : null}
      <Button variant="ghost" onClick={onCancel}>
        {t('themeEditor.cancel')}
      </Button>
      <span className="flex-1" />
      <Button variant="primary" type="submit" form={THEME_FORM_ID} iconStart="check">
        {t('themeEditor.save')}
      </Button>
    </>
  );
};
