import * as Dialog from '@radix-ui/react-dialog';
import { AnimatePresence, LazyMotion, m } from 'motion/react';
import type { ReactNode } from 'react';

import { useDocumentDirection } from '@/Core/Hooks/useDocumentDirection';
import { useFocusReturn } from '@/Core/Hooks/useFocusReturn';
import { useOverlayTransition } from '@/Core/Hooks/useOverlayTransition';
import { IconButton } from '@/Core/Ui/IconButton';

export interface ISlideOverProps {
  open: boolean;
  onClose: () => void;
  title: string;
  subtitle?: string;
  closeLabel: string;
  footer?: ReactNode;
  children: ReactNode;
}

const loadFeatures = () => import('@/Core/Ui/MotionFeatures').then((module) => module.default);

export const SlideOver = ({ open, onClose, title, subtitle, closeLabel, footer, children }: ISlideOverProps) => {
  const dir = useDocumentDirection();
  const transition = useOverlayTransition(0.32);
  const focus = useFocusReturn();
  const offscreen = dir === 'rtl' ? '-110%' : '110%';

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <Dialog.Portal forceMount>
        <LazyMotion features={loadFeatures} strict>
          <AnimatePresence>
            {open ? (
              <m.div
                key="backdrop"
                role="presentation"
                aria-hidden="true"
                onClick={onClose}
                className="pointer-events-auto fixed inset-0 z-40 bg-backdrop backdrop-blur-[8px]"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={transition}
              />
            ) : null}
            {open ? (
              <Dialog.Content
                key="panel"
                asChild
                forceMount
                {...(subtitle ? {} : { 'aria-describedby': undefined })}
                onOpenAutoFocus={focus.onOpenAutoFocus}
                onCloseAutoFocus={focus.onCloseAutoFocus}
              >
                <m.aside
                  className={
                    'fixed inset-y-3 end-3 z-50 grid w-[min(var(--spacing-panel),calc(100dvw-1.5rem))] ' +
                    'grid-rows-[auto_1fr_auto] overflow-hidden rounded-xl bg-card-gradient ' +
                    '[box-shadow:var(--shadow-card),var(--shadow-overlay)]'
                  }
                  initial={{ x: offscreen }}
                  animate={{ x: 0 }}
                  exit={{ x: offscreen }}
                  transition={transition}
                >
                  <header className="grid gap-1.5 px-6 pt-5 pb-3.5">
                    <div className="flex items-center justify-between gap-2">
                      <Dialog.Title className="m-0 text-title-xl">{title}</Dialog.Title>
                      <IconButton icon="close" label={closeLabel} onClick={onClose} className="[--edge-fill:var(--color-surface)]" />
                    </div>
                    {subtitle ? (
                      <Dialog.Description className="m-0 text-body-sm text-tertiary">{subtitle}</Dialog.Description>
                    ) : null}
                  </header>
                  <div className="grid content-start gap-5 overflow-auto px-6 pt-2 pb-5">{children}</div>
                  {footer ? (
                    <footer className="flex justify-end gap-2 border-t border-subtle px-6 py-4">{footer}</footer>
                  ) : null}
                </m.aside>
              </Dialog.Content>
            ) : null}
          </AnimatePresence>
        </LazyMotion>
      </Dialog.Portal>
    </Dialog.Root>
  );
};
