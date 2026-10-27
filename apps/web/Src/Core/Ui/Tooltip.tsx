import * as RadixTooltip from '@radix-ui/react-tooltip';
import type { ReactElement, ReactNode } from 'react';

import { useDocumentDirection } from '@/Core/Hooks/useDocumentDirection';
import { cx } from '@/Core/Ui/ClassNames';
import type { TDirection } from '@/I18n/Locale';

export interface ITooltipProps {
  label: string;
  side?: 'start' | 'end' | 'top' | 'bottom';
  phoneOnly?: boolean;
  children: ReactElement;
}

export const TooltipProvider = ({ children }: { children: ReactNode }) => (
  <RadixTooltip.Provider delayDuration={200} skipDelayDuration={300}>
    {children}
  </RadixTooltip.Provider>
);

const physicalSide = (side: NonNullable<ITooltipProps['side']>, dir: TDirection): 'left' | 'right' | 'top' | 'bottom' => {
  if (side === 'top' || side === 'bottom') return side;
  const startIsLeft = dir === 'ltr';
  if (side === 'start') return startIsLeft ? 'left' : 'right';
  return startIsLeft ? 'right' : 'left';
};

export const Tooltip = ({ label, side = 'top', phoneOnly = false, children }: ITooltipProps) => {
  const dir = useDocumentDirection();
  return (
    <RadixTooltip.Root>
      <RadixTooltip.Trigger asChild>{children}</RadixTooltip.Trigger>
      <RadixTooltip.Portal>
        <RadixTooltip.Content
          side={physicalSide(side, dir)}
          sideOffset={8}
          className={cx(
            'z-50 rounded-sm border border-line-2 bg-surface-top px-3 py-1.5 text-caption whitespace-nowrap text-primary',
            phoneOnly && 'md:hidden',
          )}
        >
          {label}
        </RadixTooltip.Content>
      </RadixTooltip.Portal>
    </RadixTooltip.Root>
  );
};
