import { AuthPage } from '@/Core/Ui/AuthPage';
import { useT } from '@/I18n/useT';
import { LoginForm } from '@/Modules/Session/Components/LoginForm';

export interface ILoginPageProps {
  onSignedIn: () => void;
}

export const LoginPage = ({ onSignedIn }: ILoginPageProps) => {
  const t = useT();

  return (
    <AuthPage title={t('login.title')} lead={t('login.lead')} footer={t('login.footer')}>
      <LoginForm onSignedIn={onSignedIn} />
    </AuthPage>
  );
};
