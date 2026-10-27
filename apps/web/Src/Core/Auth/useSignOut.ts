import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';

import { isApiError, isUnauthenticated } from '@/Core/Api/ApiError';
import { signOut } from '@/Core/Auth/AuthApi';
import { useAction } from '@/Core/Hooks/useAction';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useToastStore } from '@/Core/Store/useToastStore';
import { useT } from '@/I18n/useT';

export interface ISignOut {
  SignOut: () => Promise<void>;
  Pending: boolean;
}

export const useSignOut = (): ISignOut => {
  const client = useQueryClient();
  const navigate = useNavigate();
  const t = useT();
  const errorText = useApiErrorText();
  const Push = useToastStore((state) => state.Push);

  const action = useAction(async (): Promise<boolean> => {
    try {
      await signOut();
    } catch (error) {
      if (!isUnauthenticated(error)) throw error;
    }
    await navigate({ to: '/login', replace: true });
    client.clear();
    Push({ Id: 'signed-out', Accent: 'live', Title: t('account.signOut.done') });
    return true;
  });

  const SignOut = async () => {
    const result = await action.Run();
    if (isApiError(result)) {
      Push({ Id: 'sign-out-failed', Accent: 'failed', Title: t('account.signOut.failed'), Detail: errorText(result) });
    }
  };

  return { SignOut, Pending: action.Pending };
};
