import * as Dialog from '@radix-ui/react-dialog';
import { AnimatePresence, LazyMotion, m } from 'motion/react';
import { useEffect, useId, useState, type KeyboardEvent } from 'react';

import { useFocusReturn } from '@/Core/Hooks/useFocusReturn';
import { useOverlayTransition } from '@/Core/Hooks/useOverlayTransition';
import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';

export interface ICommandItem {
  id: string;
  label: string;
  icon?: TIconName;
  kind?: string;
}

export interface ICommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  items: ICommandItem[];
  onSelect: (item: ICommandItem) => void;
  label: string;
  placeholder: string;
  emptyText: string;
}

type TBodyProps = Omit<ICommandPaletteProps, 'open' | 'onOpenChange'>;

const loadFeatures = () => import('@/Core/Ui/MotionFeatures').then((module) => module.default);

const PaletteBody = ({ items, onSelect, label, placeholder, emptyText }: TBodyProps) => {
  const listId = useId();
  const [Query, setQuery] = useState('');
  const [Active, setActive] = useState(0);
  const needle = Query.trim().toLowerCase();
  const visible = items.filter((item) => item.label.toLowerCase().includes(needle));
  const current = visible[Active];
  const optionId = (item: ICommandItem) => `${listId}-${item.id}`;

  const activeId = current ? optionId(current) : undefined;

  useEffect(() => {
    if (activeId) document.getElementById(activeId)?.scrollIntoView({ block: 'nearest' });
  }, [activeId]);

  const HandleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      const step = event.key === 'ArrowDown' ? 1 : -1;
      setActive((index) => (visible.length ? (index + step + visible.length) % visible.length : 0));
    } else if (event.key === 'Enter' && current) {
      event.preventDefault();
      onSelect(current);
    }
  };

  return (
    <>
      <div className="flex items-center gap-2.5 border-b border-subtle bg-input px-[18px] text-tertiary">
        <Icon name="search" size={18} />
        <input
          role="combobox"
          aria-label={label}
          aria-expanded="true"
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={activeId}
          value={Query}
          placeholder={placeholder}
          onChange={(event) => {
            setQuery(event.target.value);
            setActive(0);
          }}
          onKeyDown={HandleKeyDown}
          className="h-[54px] flex-1 border-0 bg-transparent text-[15px] text-primary outline-none placeholder:text-tertiary"
        />
      </div>
      <div id={listId} role="listbox" aria-label={label} className="grid max-h-80 gap-0.5 overflow-auto p-2 empty:hidden">
        {visible.map((item, index) => (
          <div key={item.id} role="presentation">
            <button
              type="button"
              id={optionId(item)}
              role="option"
              tabIndex={-1}
              aria-selected={index === Active}
              onMouseEnter={() => {
                setActive(index);
              }}
              onClick={() => {
                onSelect(item);
              }}
              className={cx(
                'flex w-full cursor-pointer items-center gap-3 rounded-md border-0 bg-transparent px-3 py-2.5 text-start text-[14px] text-secondary',
                index === Active && 'bg-surface-raised text-primary',
              )}
            >
              {item.icon ? <Icon name={item.icon} size={17} /> : null}
              <span className="flex-1">{item.label}</span>
              {item.kind ? <small className="text-caption text-tertiary">{item.kind}</small> : null}
            </button>
          </div>
        ))}
      </div>
      {visible.length === 0 ? (
        <p role="status" className="m-0 px-5 pb-4 text-caption text-tertiary">
          {emptyText}
        </p>
      ) : null}
    </>
  );
};

export const CommandPalette = ({ open, onOpenChange, ...body }: ICommandPaletteProps) => {
  const transition = useOverlayTransition(0.2);
  const focus = useFocusReturn();

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal forceMount>
        <LazyMotion features={loadFeatures} strict>
          <AnimatePresence>
            {open ? (
              <m.div
                key="backdrop"
                role="presentation"
                aria-hidden="true"
                onClick={() => {
                  onOpenChange(false);
                }}
                className="pointer-events-auto fixed inset-0 z-40 bg-backdrop backdrop-blur-[8px]"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={transition}
              />
            ) : null}
            {open ? (
              <Dialog.Content
                key="palette"
                asChild
                forceMount
                aria-describedby={undefined}
                onOpenAutoFocus={focus.onOpenAutoFocus}
                onCloseAutoFocus={focus.onCloseAutoFocus}
              >
                <m.div
                  className={
                    'fixed inset-x-4 top-[12vh] z-50 mx-auto w-[min(580px,calc(100dvw-2rem))] overflow-hidden rounded-xl ' +
                    'bg-card-gradient [box-shadow:var(--shadow-card),var(--shadow-overlay)]'
                  }
                  initial={{ opacity: 0, y: 8, scale: 0.98 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: 8, scale: 0.98 }}
                  transition={transition}
                >
                  <Dialog.Title className="sr-only">{body.label}</Dialog.Title>
                  <PaletteBody {...body} />
                </m.div>
              </Dialog.Content>
            ) : null}
          </AnimatePresence>
        </LazyMotion>
      </Dialog.Portal>
    </Dialog.Root>
  );
};
