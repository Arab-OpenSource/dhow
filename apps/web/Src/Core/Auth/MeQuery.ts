import { ApiClient, request } from '@/Core/Api/Client';
import type { TMe } from '@/Core/Auth/AuthTypes';
import { useCsrfStore } from '@/Core/Store/useCsrfStore';

export const ME_QUERY_KEY = ['auth', 'me'] as const;

export const fetchMe = async (): Promise<TMe> => {
  const { CsrfToken, ...me } = await request(() => ApiClient.GET('/me'));
  useCsrfStore.getState().Set(CsrfToken);
  return me;
};
