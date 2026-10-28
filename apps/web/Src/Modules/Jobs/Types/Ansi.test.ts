import { describe, expect, it } from 'vitest';

import { parseAnsi, PLAIN_STYLE } from '@/Modules/Jobs/Types/Ansi';

const ESC = '\u001b';

describe('parseAnsi', () => {
  it.each([
    ['plain text', 'hello', [{ Text: 'hello', ClassName: '' }]],
    ['a red word', `a ${ESC}[31mred${ESC}[0m b`, [{ Text: 'a ', ClassName: '' }, { Text: 'red', ClassName: 'text-accent-coral' }, { Text: ' b', ClassName: '' }]],
    ['bold bright green', `${ESC}[1;92mok`, [{ Text: 'ok', ClassName: 'text-accent-green font-semibold' }]],
    ['256 colors are dropped, not misread', `${ESC}[38;5;196mx${ESC}[39my`, [{ Text: 'xy', ClassName: '' }]],
    ['cursor moves and titles are removed', `${ESC}[2K${ESC}]0;title\u0007${ESC}[1Gdone`, [{ Text: 'done', ClassName: '' }]],
    ['other control characters are removed, tabs kept', 'a\u0000\tb\u0008', [{ Text: 'a\tb', ClassName: '' }]],
  ])('handles %s', (_name, input, expected) => {
    expect(parseAnsi(input).Segments).toEqual(expected);
  });

  it('carries the style to the next call and resets with a bare SGR', () => {
    const first = parseAnsi(`${ESC}[33mwarn`);
    expect(first.Style.Fg).toBe('yellow');
    const second = parseAnsi(`still${ESC}[m plain`, first.Style);
    expect(second.Segments).toEqual([
      { Text: 'still', ClassName: 'text-accent-amber' },
      { Text: ' plain', ClassName: '' },
    ]);
    expect(second.Style).toEqual(PLAIN_STYLE);
  });
});
