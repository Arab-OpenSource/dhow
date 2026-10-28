import type { TJobLogChunk, TJobLogStream } from '@/Modules/Jobs/Api/JobsApi';
import { parseAnsi, PLAIN_STYLE, type IAnsiSegment, type IAnsiStyle } from '@/Modules/Jobs/Types/Ansi';

export const MAX_LOG_LINES = 5000;
export const MAX_LINE_LENGTH = 8192;

export type TLogLineKind = 'text' | 'step' | 'system';

export interface ILogLine {
  Id: number;
  Kind: TLogLineKind;
  Stream: TJobLogStream;
  Step: number | undefined;
  Raw: string;
  StartStyle: IAnsiStyle;
  Segments: IAnsiSegment[];
}

export interface ILogBuffer {
  Lines: ILogLine[];
  NextId: number;
  Open: boolean;
  Style: IAnsiStyle;
  Step: number | undefined;
  LastSeq: number;
  Dropped: number;
}

export const EMPTY_LOG: ILogBuffer = {
  Lines: [],
  NextId: 1,
  Open: false,
  Style: PLAIN_STYLE,
  Step: undefined,
  LastSeq: 0,
  Dropped: 0,
};

const visible = (raw: string): string => {
  const trimmed = raw.endsWith('\r') ? raw.slice(0, -1) : raw;
  const at = trimmed.lastIndexOf('\r');
  return at < 0 ? trimmed : trimmed.slice(at + 1);
};

interface IDraft {
  lines: ILogLine[];
  nextId: number;
  open: boolean;
  style: IAnsiStyle;
  step: number | undefined;
}

const render = (draft: IDraft, line: ILogLine): ILogLine => {
  const { Segments, Style } = parseAnsi(visible(line.Raw), line.StartStyle);
  draft.style = Style;
  return { ...line, Segments };
};

const startLine = (draft: IDraft, kind: TLogLineKind, stream: TJobLogStream, step: number | undefined, raw: string) => {
  const line: ILogLine = { Id: draft.nextId, Kind: kind, Stream: stream, Step: step, Raw: raw, StartStyle: draft.style, Segments: [] };
  draft.nextId += 1;
  draft.lines.push(render(draft, line));
};

const extendOpen = (draft: IDraft, raw: string) => {
  const at = draft.lines.length - 1;
  const current = draft.lines[at];
  if (current === undefined) return;
  draft.lines[at] = render(draft, { ...current, Raw: current.Raw + raw });
};

const writePart = (draft: IDraft, chunk: TJobLogChunk, part: string, continues: boolean) => {
  if (continues) extendOpen(draft, part);
  else startLine(draft, chunk.Stream === 'system' ? 'system' : 'text', chunk.Stream, chunk.StepIndex, part);
};

const splitLong = (part: string): string[] => {
  if (part.length <= MAX_LINE_LENGTH) return [part];
  const pieces: string[] = [];
  for (let at = 0; at < part.length; at += MAX_LINE_LENGTH) pieces.push(part.slice(at, at + MAX_LINE_LENGTH));
  return pieces;
};

const appendOne = (draft: IDraft, chunk: TJobLogChunk) => {
  const last = draft.lines.at(-1);
  const sameSource = last?.Stream === chunk.Stream && last.Step === chunk.StepIndex;
  if (chunk.StepIndex !== undefined && chunk.StepIndex !== draft.step) {
    draft.open = false;
    draft.step = chunk.StepIndex;
    startLine(draft, 'step', chunk.Stream, chunk.StepIndex, '');
  }
  let continues = draft.open && sameSource && chunk.Stream !== 'system';
  const parts = chunk.Text.split('\n');
  parts.forEach((part, index) => {
    const end = index === parts.length - 1;
    if (end && part === '') {
      draft.open = false;
      return;
    }
    const current = continues ? (draft.lines.at(-1)?.Raw.length ?? 0) : 0;
    const pieces = current + part.length > MAX_LINE_LENGTH ? splitLong(part) : [part];
    pieces.forEach((piece, at) => {
      writePart(draft, chunk, piece, continues && at === 0 && current + piece.length <= MAX_LINE_LENGTH);
      continues = false;
    });
    draft.open = end;
    continues = false;
  });
};

export const appendChunks = (buffer: ILogBuffer, chunks: readonly TJobLogChunk[]): ILogBuffer => {
  const fresh = chunks.filter((chunk) => chunk.Seq > buffer.LastSeq);
  if (fresh.length === 0) return buffer;
  const draft: IDraft = {
    lines: [...buffer.Lines],
    nextId: buffer.NextId,
    open: buffer.Open,
    style: buffer.Style,
    step: buffer.Step,
  };
  let lastSeq = buffer.LastSeq;
  for (const chunk of fresh) {
    if (chunk.Seq <= lastSeq) continue;
    appendOne(draft, chunk);
    lastSeq = chunk.Seq;
  }
  const excess = Math.max(0, draft.lines.length - MAX_LOG_LINES);
  return {
    Lines: excess > 0 ? draft.lines.slice(excess) : draft.lines,
    NextId: draft.nextId,
    Open: draft.open,
    Style: draft.style,
    Step: draft.step,
    LastSeq: lastSeq,
    Dropped: buffer.Dropped + excess,
  };
};

const SYSTEM_CODE = /^dhow: ([A-Z_]+):/;

export const systemCodeOf = (raw: string): string | null => SYSTEM_CODE.exec(raw)?.[1] ?? null;
