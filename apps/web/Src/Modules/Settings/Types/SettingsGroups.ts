import type { IGroupSpec } from '@/Modules/Settings/Types/SettingsTypes';

const ON_OFF = [
  { Value: 'true', Label: 'settings.choice.on' },
  { Value: 'false', Label: 'settings.choice.off' },
] as const;

export const SETTINGS_GROUPS: readonly IGroupSpec[] = [
  {
    Key: 'Mail',
    Path: 'mail',
    Title: 'settings.mail.title',
    Lead: 'settings.mail.lead',
    Icon: 'mail',
    Confirm: 'settings.mail.confirm',
    Fields: [
      { Name: 'Host', Kind: 'text', Label: 'settings.mail.host', Hint: 'settings.mail.host.hint', Ltr: true },
      { Name: 'Port', Kind: 'number', Label: 'settings.mail.port', Ltr: true },
      {
        Name: 'Tls',
        Kind: 'choice',
        Label: 'settings.mail.tls',
        Choices: [
          { Value: 'starttls', Label: 'settings.mail.tls.starttls' },
          { Value: 'tls', Label: 'settings.mail.tls.tls' },
          { Value: 'none', Label: 'settings.mail.tls.none' },
        ],
      },
      { Name: 'Username', Kind: 'text', Label: 'settings.mail.username', Ltr: true },
      { Name: 'Password', Kind: 'secret', Label: 'settings.mail.password' },
      {
        Name: 'From',
        Kind: 'text',
        Label: 'settings.mail.from',
        Hint: 'settings.mail.from.hint',
        HintParams: { example: '\u2066Dhow <dhow@example.com>\u2069' },
        Ltr: true,
      },
    ],
  },
  {
    Key: 'Organizations',
    Path: 'organizations',
    Title: 'settings.organizations.title',
    Lead: 'settings.organizations.lead',
    Icon: 'building',
    Confirm: 'settings.organizations.confirm',
    Fields: [
      {
        Name: 'Creation',
        Kind: 'choice',
        Label: 'settings.organizations.creation',
        Choices: [
          { Value: 'instance-owner', Label: 'settings.organizations.creation.owner' },
          { Value: 'any-user', Label: 'settings.organizations.creation.anyone' },
        ],
      },
    ],
  },
  {
    Key: 'Passwords',
    Path: 'passwords',
    Title: 'settings.passwords.title',
    Lead: 'settings.passwords.lead',
    Icon: 'key',
    Confirm: 'settings.passwords.confirm',
    Fields: [{ Name: 'BreachedCheck', Kind: 'choice', Label: 'settings.passwords.breached', Choices: ON_OFF }],
  },
  {
    Key: 'RateLimits',
    Path: 'rate-limits',
    Title: 'settings.rateLimits.title',
    Lead: 'settings.rateLimits.lead',
    Icon: 'shield',
    Confirm: 'settings.rateLimits.confirm',
    Fields: [
      { Name: 'Public', Kind: 'number', Label: 'settings.rateLimits.public', Hint: 'settings.rateLimits.public.hint' },
      { Name: 'InvitesPerUser', Kind: 'number', Label: 'settings.rateLimits.user', Hint: 'settings.rateLimits.hour' },
      { Name: 'InvitesPerOrganization', Kind: 'number', Label: 'settings.rateLimits.organization', Hint: 'settings.rateLimits.hour' },
    ],
  },
  {
    Key: 'Logs',
    Path: 'logs',
    Title: 'settings.logs.title',
    Lead: 'settings.logs.lead',
    Icon: 'file',
    Confirm: 'settings.logs.confirm',
    Fields: [
      { Name: 'RetentionDays', Kind: 'number', Label: 'settings.logs.retention', Hint: 'settings.logs.retention.hint' },
      { Name: 'JobLogMaxBytes', Kind: 'number', Label: 'settings.logs.maxBytes', Hint: 'settings.logs.maxBytes.hint' },
    ],
  },
];
