import { grantableRoles, type TBindableRole, type TRole } from '@/Core/Auth/AuthTypes';
import { ChoiceList, type IChoice } from '@/Core/Ui/ChoiceList';
import { useT } from '@/I18n/useT';

export const NO_ROLE = 'none';
export type TTeamRoleChoice = TBindableRole | typeof NO_ROLE;

export interface ITeamRolePickerProps {
  actor: TRole;
  value: TTeamRoleChoice;
  onChange: (value: TTeamRoleChoice) => void;
  disabled?: boolean;
}

export const TeamRolePicker = ({ actor, value, onChange, disabled }: ITeamRolePickerProps) => {
  const t = useT();
  const choices: IChoice<TTeamRoleChoice>[] = [
    { value: NO_ROLE, title: t('team.role.none'), about: t('team.role.noneAbout') },
    ...grantableRoles(actor).map((role) => ({ value: role, title: t(`role.${role}`), about: t(`role.${role}.about`) })),
  ];
  return <ChoiceList label={t('team.role')} choices={choices} value={value} onChange={onChange} disabled={disabled} />;
};
