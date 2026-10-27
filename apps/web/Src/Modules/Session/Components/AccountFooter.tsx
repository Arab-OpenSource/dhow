import { useSignOut } from '@/Core/Auth/useSignOut';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';

export interface IAccountFooterProps {
  onChangePassword: () => void;
}

export const AccountFooter = ({ onChangePassword }: IAccountFooterProps) => {
  const t = useT();
  const { SignOut, Pending } = useSignOut();

  return (
    <>
      <Button variant="danger" iconStart="out" disabled={Pending} onClick={() => void SignOut()}>
        {t('account.signOut')}
      </Button>
      <span className="flex-1" />
      <Button variant="primary" onClick={onChangePassword}>
        {t('account.password.change')}
      </Button>
    </>
  );
};
