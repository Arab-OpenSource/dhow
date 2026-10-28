import { useCallback, useEffect, useRef, useState } from 'react';

import { jobLogStreamUrl, type TJobLogChunk, type TJobLogStatus } from '@/Modules/Jobs/Api/JobsApi';
import { appendChunks, EMPTY_LOG, type ILogBuffer } from '@/Modules/Jobs/Types/LogBuffer';

export type TLogConnection = 'connecting' | 'live' | 'reconnecting' | 'ended' | 'failed';

export interface IJobLog {
  Buffer: ILogBuffer;
  Connection: TLogConnection;
  Status: TJobLogStatus | null;
  Retry: () => void;
}

export interface IJobLogTarget {
  organizationId: string;
  jobId: string;
  attempt: number;
  onEnded?: () => void;
}

export const FLUSH_MS = 120;
export const MAX_FAILURES = 5;
const BACKOFF_MS = [1000, 2000, 4000, 8000, 15000];

const parse = (event: Event): unknown => {
  if (!(event instanceof MessageEvent) || typeof event.data !== 'string') return null;
  try {
    return JSON.parse(event.data) as unknown;
  } catch {
    return null;
  }
};

export const useJobLog = ({ organizationId, jobId, attempt, onEnded }: IJobLogTarget): IJobLog => {
  const [Buffer, setBuffer] = useState<ILogBuffer>(EMPTY_LOG);
  const [Connection, setConnection] = useState<TLogConnection>('connecting');
  const [Status, setStatus] = useState<TJobLogStatus | null>(null);
  const [Generation, setGeneration] = useState(0);
  const lastSeq = useRef(0);
  const endedRef = useRef(onEnded);

  useEffect(() => {
    endedRef.current = onEnded;
  }, [onEnded]);

  useEffect(() => {
    let source: EventSource | null = null;
    let pending: TJobLogChunk[] = [];
    let flushTimer: ReturnType<typeof setTimeout> | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | null = null;
    let failures = 0;
    let stopped = false;

    const flush = () => {
      flushTimer = null;
      if (pending.length === 0) return;
      const batch = pending;
      pending = [];
      setBuffer((buffer) => appendChunks(buffer, batch));
    };

    const onChunk = (event: Event) => {
      const chunk = parse(event) as TJobLogChunk | null;
      if (chunk === null || chunk.Seq <= lastSeq.current) return;
      lastSeq.current = chunk.Seq;
      pending.push(chunk);
      flushTimer ??= setTimeout(flush, FLUSH_MS);
    };

    const onStatus = (event: Event) => {
      const status = parse(event) as TJobLogStatus | null;
      failures = 0;
      setConnection('live');
      if (status !== null) setStatus(status);
    };

    const onEnd = (event: Event) => {
      const status = parse(event) as TJobLogStatus | null;
      if (status !== null) setStatus(status);
      source?.close();
      stopped = true;
      flush();
      setConnection('ended');
      endedRef.current?.();
    };

    const connect = () => {
      retryTimer = null;
      source = new EventSource(jobLogStreamUrl(organizationId, jobId, attempt, lastSeq.current + 1), { withCredentials: true });
      source.addEventListener('chunk', onChunk);
      source.addEventListener('status', onStatus);
      source.addEventListener('end', onEnd);
      source.onopen = () => {
        setConnection('live');
      };
      source.onerror = () => {
        if (stopped || source === null) return;
        if (source.readyState !== EventSource.CLOSED) {
          setConnection('reconnecting');
          return;
        }
        source.close();
        failures += 1;
        if (failures > MAX_FAILURES) {
          setConnection('failed');
          return;
        }
        setConnection('reconnecting');
        retryTimer = setTimeout(connect, BACKOFF_MS[Math.min(failures, BACKOFF_MS.length) - 1]);
      };
    };

    connect();
    return () => {
      stopped = true;
      source?.close();
      if (flushTimer !== null) clearTimeout(flushTimer);
      if (retryTimer !== null) clearTimeout(retryTimer);
      flush();
    };
  }, [organizationId, jobId, attempt, Generation]);

  const Retry = useCallback(() => {
    setConnection('connecting');
    setGeneration((value) => value + 1);
  }, []);

  return { Buffer, Connection, Status, Retry };
};
