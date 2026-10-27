import { act, render, screen } from '@testing-library/react';
import { Suspense, useState, type ComponentType } from 'react';
import { describe, expect, it, vi } from 'vitest';

import { lazyComponent } from '@/Core/Ui/LazyComponent';
import { Skeleton } from '@/Core/Ui/Skeleton';

interface IGreetingProps {
  name: string;
}

const Greeting = ({ name }: IGreetingProps) => {
  const [Count, setCount] = useState(0);
  return (
    <button
      type="button"
      onClick={() => {
        setCount((value) => value + 1);
      }}
    >
      {`${name} ${Count}`}
    </button>
  );
};

const deferred = () => {
  let resolve: (component: ComponentType<IGreetingProps>) => void = () => undefined;
  const promise = new Promise<ComponentType<IGreetingProps>>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};

const mount = (Lazy: ComponentType<IGreetingProps>) =>
  render(
    <Suspense fallback={<Skeleton label="loading" />}>
      <Lazy name="Sabry" />
    </Suspense>,
  );

describe('lazyComponent', () => {
  it('shows the Suspense fallback until its chunk arrives, then the component with its props', async () => {
    const chunk = deferred();
    const Lazy = lazyComponent(() => chunk.promise);
    mount(Lazy);
    expect(screen.getByRole('status', { name: 'loading' })).toBeInTheDocument();
    await act(async () => {
      chunk.resolve(Greeting);
      await chunk.promise;
    });
    expect(await screen.findByRole('button', { name: 'Sabry 0' })).toBeInTheDocument();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('renders at once, with no fallback, when it was preloaded before it mounts', async () => {
    const load = vi.fn(() => Promise.resolve(Greeting));
    const Lazy = lazyComponent(load);
    await Lazy.preload();
    mount(Lazy);
    expect(screen.getByRole('button', { name: 'Sabry 0' })).toBeInTheDocument();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('keeps the mounted instance and its state when a later preload finishes', async () => {
    const chunk = deferred();
    const Lazy = lazyComponent(() => chunk.promise);
    const view = mount(Lazy);
    await act(async () => {
      chunk.resolve(Greeting);
      await chunk.promise;
    });
    const button = await screen.findByRole('button', { name: 'Sabry 0' });
    act(() => {
      button.click();
    });
    await Lazy.preload();
    view.rerender(
      <Suspense fallback={<Skeleton label="loading" />}>
        <Lazy name="Sabry" />
      </Suspense>,
    );
    expect(screen.getByRole('button', { name: 'Sabry 1' })).toBe(button);
  });

  it('never rejects a preload, so a lost chunk does not raise an unhandled rejection', async () => {
    const Lazy = lazyComponent<IGreetingProps>(() => Promise.reject(new Error('chunk is gone')));
    await expect(Lazy.preload()).resolves.toBeUndefined();
  });
});
