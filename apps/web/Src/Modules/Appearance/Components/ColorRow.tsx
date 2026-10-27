import { useState } from 'react';

import { isHex } from '@/Core/Theme/ThemeValidation';
import { useT } from '@/I18n/useT';

export interface IColorRowProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
}

export const ColorRow = ({ label, value, onChange }: IColorRowProps) => {
  const t = useT();
  const [Text, setText] = useState(value);
  const [Seen, setSeen] = useState(value);
  const invalid = !isHex(Text);

  if (value !== Seen) {
    setSeen(value);
    setText(value);
  }

  return (
    <div className="flex items-center gap-2.5 rounded-md bg-row py-1.5 ps-3 pe-2 text-caption">
      <span className="min-w-0 flex-1">{label}</span>
      <input
        type="text"
        dir="ltr"
        spellCheck={false}
        autoComplete="off"
        value={Text}
        aria-label={t('themeEditor.hexLabel', { name: label })}
        aria-invalid={invalid ? true : undefined}
        onChange={(event) => {
          const next = event.target.value.trim();
          setText(event.target.value);
          if (isHex(next)) onChange(next.toLowerCase());
        }}
        className={
          'h-[34px] w-[92px] rounded-[10px] border border-default bg-input px-2 font-mono text-micro text-primary ' +
          'aria-invalid:border-accent-coral'
        }
      />
      <input
        type="color"
        value={value}
        aria-label={label}
        onChange={(event) => {
          onChange(event.target.value.toLowerCase());
        }}
        className={
          'size-[34px] cursor-pointer rounded-[10px] border-0 bg-transparent p-0 ' +
          '[&::-webkit-color-swatch]:rounded-[10px] [&::-webkit-color-swatch]:border [&::-webkit-color-swatch]:border-strong ' +
          '[&::-webkit-color-swatch-wrapper]:p-0'
        }
      />
    </div>
  );
};
