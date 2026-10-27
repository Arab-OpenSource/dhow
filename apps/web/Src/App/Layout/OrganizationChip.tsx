import { preloadFrame } from '@/App/Layout/LazyFrames';
import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';

export interface IOrganizationChipProps {
  className?: string;
}

const preloadOrganization = () => {
  void preloadFrame('organization');
};

export const OrganizationChip = ({ className }: IOrganizationChipProps) => {
  const { openStack } = useShellNavigation();
  const membership = useCurrentMembership();
  if (membership === undefined) return null;

  return (
    <button
      type="button"
      onPointerEnter={preloadOrganization}
      onFocus={preloadOrganization}
      onClick={() => {
        openStack(['organization']);
      }}
      className={cx(
        'flex min-w-0 max-w-full cursor-pointer items-center gap-2 rounded-sm border border-line bg-transparent px-2.5 py-1.5',
        'text-start text-body-sm text-secondary transition-colors duration-140 hover:bg-hover hover:text-primary',
        className,
      )}
    >
      <Icon name="building" size={16} />
      <span className="truncate">{membership.OrganizationName}</span>
    </button>
  );
};
