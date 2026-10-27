import type { QueryClient } from '@tanstack/react-query';

import { ApiClient, request, requestVoid } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';
import { useCsrfStore } from '@/Core/Store/useCsrfStore';

export type TLoginResult = components['schemas']['LoginResult'];

export interface ICredentials {
  Email: string;
  Password: string;
}

export const signIn = async (client: QueryClient, credentials: ICredentials): Promise<TLoginResult> => {
  const result = await request(() => ApiClient.POST('/sessions', { body: credentials }));
  client.clear();
  useCsrfStore.getState().Set(result.CsrfToken);
  return result;
};

export const signOut = async (): Promise<void> => {
  await requestVoid(() => ApiClient.DELETE('/sessions/current'));
  useCsrfStore.getState().Clear();
};
