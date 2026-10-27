import { useState } from 'react';

import { isHex } from '@/Core/Theme/ThemeValidation';
import { Input } from '@/Core/Ui/Input';
import { Row } from '@/Core/Ui/Row';
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
    <Row
      title={label}
      end={
        <>
          <Input
            compact
            dir="ltr"
            spellCheck={false}
            autoComplete="off"
            value={Text}
            invalid={invalid}
            aria-label={t('themeEditor.hexLabel', { name: label })}
            onChange={(event) => {
              const next = event.target.value.trim();
              setText(event.target.value);
              if (isHex(next)) onChange(next.toLowerCase());
            }}
            className="w-24 font-mono"
          />
          <input
            type="color"
            value={value}
            aria-label={label}
            onChange={(event) => {
              onChange(event.target.value.toLowerCase());
            }}
            className={
              'size-btn-sm cursor-pointer rounded-input border-0 bg-transparent p-0 ' +
              '[&::-webkit-color-swatch]:rounded-input [&::-webkit-color-swatch]:border [&::-webkit-color-swatch]:border-line-2 ' +
              '[&::-webkit-color-swatch-wrapper]:p-0'
            }
          />
        </>
      }
    />
  );
};
