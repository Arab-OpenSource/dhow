export type TAnsiColor = 'black' | 'red' | 'green' | 'yellow' | 'blue' | 'magenta' | 'cyan' | 'white';

export interface IAnsiStyle {
  Fg: TAnsiColor | null;
  Bold: boolean;
  Dim: boolean;
  Italic: boolean;
  Underline: boolean;
}

export interface IAnsiSegment {
  Text: string;
  ClassName: string;
}

export interface IAnsiResult {
  Segments: IAnsiSegment[];
  Style: IAnsiStyle;
}

export const PLAIN_STYLE: IAnsiStyle = { Fg: null, Bold: false, Dim: false, Italic: false, Underline: false };

const COLORS: readonly TAnsiColor[] = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white'];

const COLOR_CLASS: Record<TAnsiColor, string> = {
  black: 'text-tertiary',
  red: 'text-accent-coral',
  green: 'text-accent-green',
  yellow: 'text-accent-amber',
  blue: 'text-accent-blue',
  magenta: 'text-accent-violet',
  cyan: 'text-accent-violet-soft',
  white: 'text-primary',
};

const ESC = '\u001b';
const BEL = '\u0007';
const SEQUENCE = new RegExp(
  `${ESC}\\[([0-9;?]*)([@-~])|${ESC}\\][^${BEL}${ESC}]*(?:${BEL}|${ESC}\\\\)?|${ESC}[@-Z\\\\-_]?|[\\u0000-\\u0008\\u000b-\\u001f\\u007f]`,
  'g',
);

export const classOf = (style: IAnsiStyle): string =>
  [
    style.Fg === null ? '' : COLOR_CLASS[style.Fg],
    style.Bold ? 'font-semibold' : '',
    style.Dim ? 'opacity-70' : '',
    style.Italic ? 'italic' : '',
    style.Underline ? 'underline' : '',
  ]
    .filter((part) => part !== '')
    .join(' ');

const extendedColorLength = (codes: readonly number[], at: number): number => {
  const mode = codes[at + 1];
  if (mode === 5) return 2;
  if (mode === 2) return 4;
  return 0;
};

const applyCode = (style: IAnsiStyle, code: number): IAnsiStyle => {
  if (code === 0) return PLAIN_STYLE;
  if (code === 1) return { ...style, Bold: true };
  if (code === 2) return { ...style, Dim: true };
  if (code === 3) return { ...style, Italic: true };
  if (code === 4) return { ...style, Underline: true };
  if (code === 22) return { ...style, Bold: false, Dim: false };
  if (code === 23) return { ...style, Italic: false };
  if (code === 24) return { ...style, Underline: false };
  if (code === 39) return { ...style, Fg: null };
  if (code >= 30 && code <= 37) return { ...style, Fg: COLORS[code - 30] ?? null };
  if (code >= 90 && code <= 97) return { ...style, Fg: COLORS[code - 90] ?? null };
  return style;
};

export const applySgr = (style: IAnsiStyle, params: string): IAnsiStyle => {
  const codes = params === '' ? [0] : params.split(';').map((part) => (part === '' ? 0 : Number(part)));
  let next = style;
  for (let at = 0; at < codes.length; at += 1) {
    const code = codes[at] ?? 0;
    if (code === 38 || code === 48) {
      if (code === 38) next = { ...next, Fg: null };
      at += extendedColorLength(codes, at);
      continue;
    }
    next = applyCode(next, code);
  }
  return next;
};

export const parseAnsi = (text: string, start: IAnsiStyle = PLAIN_STYLE): IAnsiResult => {
  const segments: IAnsiSegment[] = [];
  let style = start;
  let last = 0;
  const push = (until: number) => {
    if (until <= last) return;
    const piece = text.slice(last, until);
    const className = classOf(style);
    const previous = segments.at(-1);
    if (previous?.ClassName === className) previous.Text += piece;
    else segments.push({ Text: piece, ClassName: className });
  };
  for (const match of text.matchAll(SEQUENCE)) {
    push(match.index);
    last = match.index + match[0].length;
    if (match[2] === 'm' && match[1] !== undefined && !match[1].includes('?')) style = applySgr(style, match[1]);
  }
  push(text.length);
  return { Segments: segments, Style: style };
};
