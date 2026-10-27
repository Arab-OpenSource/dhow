import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';

export interface IAppearanceFooterProps {
  onCreate: () => void;
}

export const AppearanceFooter = ({ onCreate }: IAppearanceFooterProps) => {
  const t = useT();

  return (
    <>
      <span className="flex-1" />
      <Button variant="primary" iconStart="plus" onClick={onCreate}>
        {t('appearance.create')}
      </Button>
    </>
  );
};
