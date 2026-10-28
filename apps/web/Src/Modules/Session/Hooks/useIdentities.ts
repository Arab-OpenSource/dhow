import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { listOidcProviders, type TOidcProvider } from '@/Core/Auth/AuthApi';
import type { TFactorMethod } from '@/Core/Auth/Factor';
import { useFactorAction, type IFactorAction } from '@/Core/Hooks/useFactorAction';
import { useToastStore } from '@/Core/Store/useToastStore';
import { useT } from '@/I18n/useT';
import {
  beginLink,
  IDENTITIES_QUERY_KEY,
  listIdentities,
  OIDC_PROVIDERS_QUERY_KEY,
  removeIdentity,
  type TLinkedIdentity,
} from '@/Modules/Session/Api/OidcApi';
import { rememberOidcPurpose } from '@/Modules/Session/Store/OidcCallback';

export interface IIdentityEntry {
  Key: string;
  Provider: string;
  Name: string;
  Configured: boolean;
  Identity: TLinkedIdentity | null;
}

export const identityEntries = (providers: readonly TOidcProvider[], identities: readonly TLinkedIdentity[]): IIdentityEntry[] => {
  const entries: IIdentityEntry[] = providers.map((provider) => ({
    Key: provider.Id,
    Provider: provider.Id,
    Name: provider.Name,
    Configured: true,
    Identity: identities.find((identity) => identity.Provider === provider.Id) ?? null,
  }));
  for (const identity of identities) {
    if (!providers.some((provider) => provider.Id === identity.Provider)) {
      entries.push({ Key: identity.Id, Provider: identity.Provider, Name: identity.ProviderName, Configured: false, Identity: identity });
    }
  }
  return entries;
};

export interface IIdentitiesCard {
  Entries: IIdentityEntry[] | undefined;
  Failed: boolean;
  Open: string | null;
  Action: IFactorAction<unknown>;
  Ask: (key: string) => void;
  Cancel: () => void;
  Confirm: () => void;
}

const leave = (url: string): void => {
  window.location.assign(url);
};

export const useIdentitiesCard = (methods: readonly TFactorMethod[], navigate: (url: string) => void = leave): IIdentitiesCard => {
  const t = useT();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const providers = useQuery({ queryKey: OIDC_PROVIDERS_QUERY_KEY, queryFn: listOidcProviders });
  const identities = useQuery({ queryKey: IDENTITIES_QUERY_KEY, queryFn: listIdentities });
  const [Open, setOpen] = useState<string | null>(null);
  const Entries = providers.data && identities.data ? identityEntries(providers.data, identities.data) : undefined;
  const entry = Entries?.find((item) => item.Key === Open) ?? null;

  const Action = useFactorAction<unknown>(methods, async (factor) => {
    if (entry === null) return null;
    if (entry.Identity === null) {
      const authorization = await beginLink(entry.Provider, await factor());
      rememberOidcPurpose('link');
      navigate(authorization.AuthorizationUrl);
      return authorization;
    }
    const revoked = await removeIdentity(entry.Identity.Id, await factor());
    setOpen(null);
    await client.invalidateQueries({ queryKey: IDENTITIES_QUERY_KEY });
    Push({
      Id: `identity-removed-${entry.Identity.Id}`,
      Accent: 'live',
      Title: t('identities.removed', { provider: entry.Name }),
      Detail: revoked > 0 ? t('mfa.revoked') : undefined,
    });
    return revoked;
  });

  return {
    Entries,
    Failed: providers.isError || identities.isError,
    Open,
    Action,
    Ask: (key) => {
      Action.Reset();
      setOpen(key);
    },
    Cancel: () => {
      setOpen(null);
    },
    Confirm: () => {
      void Action.Run();
    },
  };
};
