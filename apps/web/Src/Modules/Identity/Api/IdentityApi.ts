import { ApiClient, request } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';

export type TSetupRequest = components['schemas']['SetupRequest'];
export type TSetupResult = components['schemas']['SetupResult'];

export const setupInstance = (body: TSetupRequest): Promise<TSetupResult> =>
  request(() => ApiClient.POST('/setup', { body }));

export type TInvitationPreview = components['schemas']['InvitationPreview'];
export type TInvitationAccepted = components['schemas']['InvitationAccepted'];
export type TAcceptInvitationRequest = components['schemas']['AcceptInvitationRequest'];

export const previewInvitation = (token: string): Promise<TInvitationPreview> =>
  request(() => ApiClient.POST('/invitations/preview', { body: { Token: token } }));

export const acceptInvitation = (body: TAcceptInvitationRequest): Promise<TInvitationAccepted> =>
  request(() => ApiClient.POST('/invitations/accept', { body }));

export const acceptInvitationSignedIn = (token: string): Promise<TInvitationAccepted> =>
  request(() => ApiClient.POST('/me/invitations/accept', { body: { Token: token } }));
