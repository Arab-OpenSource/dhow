import { useCallback, useLayoutEffect, useRef, useState, type RefObject } from 'react';

export const BOTTOM_SLACK_PX = 32;

export interface IFollowScroll {
  Ref: RefObject<HTMLDivElement | null>;
  Following: boolean;
  HandleScroll: () => void;
  JumpToEnd: () => void;
}

const atBottom = (element: HTMLElement): boolean =>
  element.scrollHeight - element.scrollTop - element.clientHeight <= BOTTOM_SLACK_PX;

export const useFollowScroll = (version: unknown): IFollowScroll => {
  const Ref = useRef<HTMLDivElement>(null);
  const [Following, setFollowing] = useState(true);

  const HandleScroll = useCallback(() => {
    const element = Ref.current;
    if (element !== null) setFollowing(atBottom(element));
  }, []);

  const JumpToEnd = useCallback(() => {
    const element = Ref.current;
    if (element !== null) element.scrollTop = element.scrollHeight;
    setFollowing(true);
  }, []);

  useLayoutEffect(() => {
    const element = Ref.current;
    if (Following && element !== null) element.scrollTop = element.scrollHeight;
  }, [Following, version]);

  return { Ref, Following, HandleScroll, JumpToEnd };
};
