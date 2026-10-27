import * as RadixTooltip from '@radix-ui/react-tooltip';
import type { ReactElement, ReactNode } from 'react';

import { useDocumentDirection } from '@/Core/Hooks/useDocumentDirection';
import type { TDirection } from '@/I18n/Locale';

export interface ITooltipProps {
  label: string;
  side?: 'start' | 'end' | 'top' | 'bottom';
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

export const Tooltip = ({ label, side = 'top', children }: ITooltipProps) => {
  const dir = useDocumentDirection();
  return (
    <RadixTooltip.Root>
      <RadixTooltip.Trigger asChild>{children}</RadixTooltip.Trigger>
      <RadixTooltip.Portal>
        <RadixTooltip.Content
          side={physicalSide(side, dir)}
          sideOffset={8}
          className="z-50 rounded-md border border-default bg-surface-top px-3 py-1.5 text-caption whitespace-nowrap text-primary"
        >
          {label}
        </RadixTooltip.Content>
      </RadixTooltip.Portal>
    </RadixTooltip.Root>
  );
};
