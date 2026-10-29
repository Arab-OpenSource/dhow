import { useNavigate, useSearch } from '@tanstack/react-router';

import { UNKNOWN_SLIDE, type TSlideId } from '@/App/Panels';

export interface ISlideNavigation {
  slide: TSlideId | null;
  invalid: boolean;
  open: (id: TSlideId) => void;
  close: () => void;
  discard: () => void;
}

export const useSlide = (): ISlideNavigation => {
  const navigate = useNavigate();
  const { slide: param } = useSearch({ strict: false });
  const invalid = param === UNKNOWN_SLIDE;

  const show = (id: TSlideId | undefined, replace = false) => {
    void navigate({ to: '.', search: (previous) => ({ ...previous, slide: id }), replace });
  };

  return {
    slide: invalid || param === undefined ? null : param,
    invalid,
    open: (id) => {
      show(id);
    },
    close: () => {
      show(undefined);
    },
    discard: () => {
      show(undefined, true);
    },
  };
};
