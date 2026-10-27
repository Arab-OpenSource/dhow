import { useState } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { IconButton } from '@/Core/Ui/IconButton';
import { Input, type IInputProps } from '@/Core/Ui/Input';
import { useT } from '@/I18n/useT';

export type TPasswordInputProps = Omit<IInputProps, 'type'>;

export const PasswordInput = ({ className, ...rest }: TPasswordInputProps) => {
  const t = useT();
  const [Visible, setVisible] = useState(false);

  return (
    <span dir="ltr" className="relative block">
      <Input type={Visible ? 'text' : 'password'} className={cx('pe-12', className)} {...rest} />
      <span className="absolute end-1 top-1/2 -translate-y-1/2">
        <IconButton
          plain
          size="sm"
          icon={Visible ? 'eyeOff' : 'eye'}
          label={Visible ? t('password.hide') : t('password.show')}
          onClick={() => {
            setVisible((current) => !current);
          }}
        />
      </span>
    </span>
  );
};
