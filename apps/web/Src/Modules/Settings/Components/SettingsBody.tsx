import { cardStateOf } from '@/Core/Ui/CardStates';
import { useT } from '@/I18n/useT';
import { MailTest } from '@/Modules/Settings/Components/MailTest';
import { SettingsCard } from '@/Modules/Settings/Components/SettingsCard';
import { useInstanceSettings } from '@/Modules/Settings/Hooks/useInstanceSettings';
import { SETTINGS_GROUPS } from '@/Modules/Settings/Types/SettingsGroups';

export const SettingsBody = () => {
  const t = useT();
  const query = useInstanceSettings();
  const state = cardStateOf(query, false);
  const mailOn = query.data !== undefined && query.data.Mail.Host.Value !== '';

  return (
    <div className="grid gap-block">
      {SETTINGS_GROUPS.map((spec) => (
        <SettingsCard
          key={spec.Key}
          spec={spec}
          settings={query.data}
          state={state}
          onRetry={() => {
            void query.refetch();
          }}
          emptyNote={spec.Key === 'Mail' ? t('settings.mail.off') : t('settings.allDefault')}
          footer={spec.Key === 'Mail' ? <MailTest enabled={mailOn} /> : undefined}
        />
      ))}
    </div>
  );
};
