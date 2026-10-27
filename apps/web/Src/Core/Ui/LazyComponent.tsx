import { lazy, useState, type ComponentType } from 'react';

export interface IPreloadable {
  preload: () => Promise<void>;
}

export type TLazyComponent<P extends object> = ComponentType<P> & IPreloadable;

export const lazyComponent = <P extends object>(load: () => Promise<ComponentType<P>>): TLazyComponent<P> => {
  const cache: { Loaded?: ComponentType<P> } = {};
  const take = (component: ComponentType<P>): ComponentType<P> => {
    cache.Loaded = component;
    return component;
  };
  const Deferred = lazy(() => load().then((component) => ({ default: take(component) })));

  const Lazy = (props: P) => {
    const [Ready] = useState(() => cache.Loaded);
    return Ready ? <Ready {...props} /> : <Deferred {...props} />;
  };

  return Object.assign(Lazy, {
    preload: () =>
      load().then(
        (component) => {
          take(component);
        },
        () => undefined,
      ),
  });
};
