import { useNavigate, useSearch } from '@tanstack/react-router';

import { UNKNOWN_STACK, type TFrameId } from '@/App/Panels';

export interface IShellNavigation {
  stack: TFrameId[];
  invalidStack: boolean;
  pushFrame: (id: TFrameId) => void;
  openStack: (ids: TFrameId[]) => void;
  popFrame: () => void;
  jumpTo: (index: number) => void;
  closeAll: () => void;
  discardStack: () => void;
  goWorkspace: () => void;
}

const NO_FRAMES: TFrameId[] = [];

export const useShellNavigation = (): IShellNavigation => {
  const navigate = useNavigate();
  const { stack: param } = useSearch({ strict: false });
  const invalidStack = param === UNKNOWN_STACK;
  const stack = invalidStack || param === undefined ? NO_FRAMES : param;

  const show = (ids: TFrameId[], replace = false) => {
    void navigate({
      to: '/',
      search: (previous) => ({ ...previous, stack: ids.length > 0 ? ids : undefined }),
      replace,
    });
  };

  return {
    stack,
    invalidStack,
    pushFrame: (id) => {
      const at = stack.indexOf(id);
      show(at >= 0 ? stack.slice(0, at + 1) : [...stack, id]);
    },
    openStack: (ids) => {
      show(ids);
    },
    popFrame: () => {
      show(stack.slice(0, -1));
    },
    jumpTo: (index) => {
      show(stack.slice(0, index + 1));
    },
    closeAll: () => {
      show([]);
    },
    discardStack: () => {
      show([], true);
    },
    goWorkspace: () => {
      show([]);
    },
  };
};
