import { describe, expect, it } from 'vitest';

import type { TJobLogChunk } from '@/Modules/Jobs/Api/JobsApi';
import { appendChunks, EMPTY_LOG, MAX_LINE_LENGTH, MAX_LOG_LINES, systemCodeOf, type ILogBuffer } from '@/Modules/Jobs/Types/LogBuffer';

const chunk = (seq: number, text: string, extra: Partial<TJobLogChunk> = {}): TJobLogChunk => ({
  Seq: seq,
  Stream: 'stdout',
  StepIndex: 0,
  CreatedAt: '2026-10-28T10:00:00Z',
  Text: text,
  ...extra,
});

const texts = (buffer: ILogBuffer) =>
  buffer.Lines.filter((line) => line.Kind !== 'step').map((line) => line.Segments.map((segment) => segment.Text).join(''));

describe('appendChunks', () => {
  it('starts each step with a header and splits lines across chunks', () => {
    const buffer = appendChunks(EMPTY_LOG, [chunk(1, 'a\nb'), chunk(2, 'c\n'), chunk(3, 'd\n', { StepIndex: 1 })]);
    expect(buffer.Lines.map((line) => line.Kind)).toEqual(['step', 'text', 'text', 'step', 'text']);
    expect(texts(buffer)).toEqual(['a', 'bc', 'd']);
    expect(buffer.LastSeq).toBe(3);
  });

  it('ignores chunks it already has, so a resumed stream never repeats a line', () => {
    const once = appendChunks(EMPTY_LOG, [chunk(1, 'a\n'), chunk(2, 'b\n')]);
    const again = appendChunks(once, [chunk(2, 'b\n'), chunk(3, 'c\n')]);
    expect(texts(again)).toEqual(['a', 'b', 'c']);
    expect(appendChunks(again, [chunk(1, 'a\n')])).toBe(again);
  });

  it('keeps stdout and stderr on separate lines and server lines on their own', () => {
    const buffer = appendChunks(EMPTY_LOG, [
      chunk(1, 'out'),
      chunk(2, 'err\n', { Stream: 'stderr' }),
      chunk(3, 'dhow: LOG_TRUNCATED: the log reached its limit', { Stream: 'system', StepIndex: undefined }),
    ]);
    expect(buffer.Lines.slice(1).map((line) => [line.Kind, line.Stream])).toEqual([
      ['text', 'stdout'],
      ['text', 'stderr'],
      ['system', 'system'],
    ]);
    expect(systemCodeOf(buffer.Lines.at(-1)?.Raw ?? '')).toBe('LOG_TRUNCATED');
  });

  it('shows only what follows the last carriage return, like a terminal', () => {
    const buffer = appendChunks(EMPTY_LOG, [chunk(1, '10%\r50%\r100%\r\n')]);
    expect(texts(buffer)).toEqual(['100%']);
  });

  it('keeps the last lines only and counts the ones it dropped', () => {
    const text = Array.from({ length: MAX_LOG_LINES + 50 }, (_, index) => `line ${String(index)}\n`).join('');
    const buffer = appendChunks(EMPTY_LOG, [chunk(1, text)]);
    expect(buffer.Lines).toHaveLength(MAX_LOG_LINES);
    expect(buffer.Dropped).toBe(51);
    expect(texts(buffer).at(-1)).toBe(`line ${String(MAX_LOG_LINES + 49)}`);
  });

  it('breaks a very long line into pieces', () => {
    const buffer = appendChunks(EMPTY_LOG, [chunk(1, 'x'.repeat(MAX_LINE_LENGTH * 2 + 5))]);
    expect(texts(buffer).map((line) => line.length)).toEqual([MAX_LINE_LENGTH, MAX_LINE_LENGTH, 5]);
  });
});
