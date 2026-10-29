import { vi } from 'vitest';

export class FakeEventSource extends EventTarget {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;
  static instances: FakeEventSource[] = [];

  readonly url: string;
  readonly withCredentials: boolean;
  readyState = FakeEventSource.CONNECTING;
  onopen: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;

  constructor(url: string | URL, init?: EventSourceInit) {
    super();
    this.url = String(url);
    this.withCredentials = init?.withCredentials ?? false;
    FakeEventSource.instances.push(this);
  }

  close() {
    this.readyState = FakeEventSource.CLOSED;
  }

  open() {
    this.readyState = FakeEventSource.OPEN;
    this.onopen?.(new Event('open'));
  }

  emit(type: string, data: unknown, id = '') {
    this.dispatchEvent(new MessageEvent(type, { data: typeof data === 'string' ? data : JSON.stringify(data), lastEventId: id }));
  }

  fail(closed: boolean) {
    this.readyState = closed ? FakeEventSource.CLOSED : FakeEventSource.CONNECTING;
    this.onerror?.(new Event('error'));
  }

  static latest(): FakeEventSource {
    const source = FakeEventSource.instances.at(-1);
    if (source === undefined) throw new Error('no EventSource was opened');
    return source;
  }
}

export const installFakeEventSource = () => {
  FakeEventSource.instances = [];
  vi.stubGlobal('EventSource', FakeEventSource);
};
