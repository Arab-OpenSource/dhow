import { useReducedMotion } from 'motion/react';

export interface IOverlayTransition {
  duration: number;
  ease: [number, number, number, number];
}

export const useOverlayTransition = (seconds: number): IOverlayTransition => {
  const reduce = useReducedMotion();
  return { duration: reduce ? 0 : seconds, ease: [0.22, 1, 0.36, 1] };
};
