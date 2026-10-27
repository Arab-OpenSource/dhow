import type { TMessages } from '@/I18n/Messages';

export const En: TMessages = {
  'app.title': 'Dhow',

  'shell.nav.label': 'Main navigation',
  'shell.nav.workspace': 'Workspace',
  'shell.nav.systemStatus': 'Server status',
  'shell.skipToContent': 'Skip to content',
  'shell.brand': 'Dhow',
  'shell.search': 'Search for a page or an action',
  'shell.notifications': 'Open notifications',
  'shell.language.switch': 'Switch the language to {language}',

  'language.ar': 'العربية',
  'language.en': 'English',

  'palette.label': 'Commands',
  'palette.placeholder': 'Type a page or an action',
  'palette.empty': 'Nothing matches. Try another word.',
  'palette.kind.page': 'Page',
  'palette.kind.panel': 'Panel',
  'palette.kind.action': 'Action',
  'palette.command.workspace': 'Go to the workspace',
  'palette.command.systemStatus': 'Open server status',
  'palette.command.notifications': 'Open notifications',
  'palette.command.language': 'Switch the language to {language}',

  'ui.close': 'Close',
  'ui.loading': 'Loading',

  'toast.dismiss': 'Dismiss',
  'toast.unknownPanel.title': 'That panel does not exist',
  'toast.unknownPanel.detail': 'We took you back to the workspace.',

  'notFound.text': 'This page does not exist.',
  'notFound.action': 'Back to the workspace',

  'workspace.title': 'Workspace',
  'workspace.empty.text': 'No projects here yet. Once you add a project, it will show up in this space.',
  'workspace.empty.action': 'Open server status',

  'notifications.title': 'Notifications',
  'notifications.empty': 'No new notifications.',

  'setup.banner.title': 'First-time setup is not done yet',
  'setup.banner.body': 'The server is running, but it still needs to finish its first-time setup.',

  'apiDown.title': 'We cannot reach the server',
  'apiDown.body': 'The interface keeps working. Check that the server is running and your network is up.',
  'apiDown.retry': 'Try the connection again',

  'status.title': 'Server status',
  'status.subtitle': 'The checks and the version running right now',
  'status.loading': 'Loading the server status',
  'status.error.title': 'We could not get the server status',
  'status.retry': 'Try again',
  'status.refresh': 'Refresh server status',
  'status.refreshing': 'Refreshing the status',
  'status.build.title': 'Version',
  'status.build.version': 'Version number',
  'status.build.commit': 'Commit',
  'status.build.setup': 'First-time setup',
  'status.build.setupDone': 'Done',
  'status.build.setupPending': 'Not done yet',
  'status.checks.title': 'Checks',
  'status.checks.empty': 'The server did not report any checks.',
  'status.check.database': 'Database',
  'status.check.migrations': 'Database migrations',
  'status.state.live': 'Running',
  'status.state.failed': 'Failed',

  'api.error.network': 'We cannot reach the server. Check your network and try again.',
  'api.error.invalid': 'The server sent a reply we do not understand. Try again.',
  'api.error.unknown': 'Something went wrong. Try again.',
};
