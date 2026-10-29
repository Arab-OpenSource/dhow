import type { PagesAr } from '@/I18n/Locales/Workspace/PagesAr';

export const PagesEn: Record<keyof typeof PagesAr, string> = {
  'page.members.lead': 'The organization’s members and the role of each.',
  'page.teams.lead': 'A team groups members and grants them one role.',
  'page.invitations.lead': 'Invitations that have not been accepted yet.',
  'page.orgSettings.lead': 'The organization’s name and its security rules.',
  'page.runnerTokens.lead': 'The tokens that register new runners in the organization.',
  'page.profile.lead': 'Your details and your personal API tokens.',
  'page.security.lead': 'Your password, second step and the devices you signed in from.',
  'page.preferences.lead': 'How the interface looks and which language it uses. Saved for you only.',
  'page.notifications.lead': 'Important alerts about jobs and the organization.',
  'preferences.language.title': 'Language',
  'preferences.language.lead': 'Choose the interface language.',

  'palette.command.members': 'Open members',
  'palette.command.teams': 'Open teams',
  'palette.command.invitations': 'Open invitations',
  'palette.command.orgSettings': 'Open organization settings',
  'palette.command.runnerTokens': 'Open registration tokens',
  'palette.command.security': 'Open security settings',
  'palette.command.preferences': 'Open preferences',
  'palette.command.newTeam': 'Create a team',
  'palette.command.newRunner': 'Add a runner',
  'palette.command.newApiKey': 'Create an organization API key',
  'palette.command.newAccountToken': 'Create a personal API key',
  'palette.command.newOrganization': 'Create an organization',
};
