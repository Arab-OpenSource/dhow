import { ApiClient, request, requestVoid } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';

export type TSessionView = components['schemas']['SessionView'];
export type TChangePassword = components['schemas']['ChangePasswordRequest'];

export const listSessions = (): Promise<TSessionView[]> => request(() => ApiClient.GET('/sessions'));

export const revokeSession = async (id: string): Promise<boolean> => {
  await requestVoid(() => ApiClient.DELETE('/sessions/{id}', { params: { path: { id } } }));
  return true;
};

export const revokeOtherSessions = async (): Promise<number> => {
  const result = await request(() => ApiClient.POST('/sessions/revoke-others'));
  return result.Revoked;
};

export const changePassword = async (body: TChangePassword): Promise<number> => {
  const result = await request(() => ApiClient.PUT('/me/password', { body }));
  return result.Revoked;
};
