import { useNavigate } from '@tanstack/react-router';

import { Icon } from '@/Core/Ui/Icon';
import { followInApp } from '@/Core/Ui/PlainClick';

export interface IBackLinkProps {
  to: '/org/members' | '/org/teams';
  label: string;
}

export const BackLink = ({ to, label }: IBackLinkProps) => {
  const navigate = useNavigate();
  return (
    <a
      href={to}
      data-back-link=""
      onClick={followInApp(() => {
        void navigate({ to });
      })}
      className="inline-flex items-center gap-1.5 justify-self-start rounded-sm text-label-sm text-secondary no-underline hover:text-primary xl:hidden"
    >
      <Icon name="back" size={16} />
      {label}
    </a>
  );
};
