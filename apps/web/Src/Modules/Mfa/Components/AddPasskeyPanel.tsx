import { useState } from 'react';

import { Field } from '@/Core/Ui/Field';
import { Input } from '@/Core/Ui/Input';
import { useT } from '@/I18n/useT';
import { addPasskey, type TAddPasskeyResult, type TFactorMethod } from '@/Modules/Mfa/Api/MfaApi';
import { ConfirmPanel } from '@/Modules/Mfa/Components/ConfirmPanel';
import { useFactorAction } from '@/Modules/Mfa/Hooks/useFactorAction';

export interface IAddPasskeyPanelProps {
  methods: readonly TFactorMethod[];
  defaultName: string;
  onAdded: (result: TAddPasskeyResult) => void;
  onCancel: () => void;
}

export const AddPasskeyPanel = ({ methods, defaultName, onAdded, onCancel }: IAddPasskeyPanelProps) => {
  const t = useT();
  const [Name, setName] = useState(defaultName);
  const add = useFactorAction(methods, (factor) => addPasskey(Name.trim(), factor));

  return (
    <div className="grid gap-3 border-t border-line pt-3">
      <Field label={t('mfa.passkeys.name')} className="px-4">
        <Input
          maxLength={64}
          value={Name}
          onChange={(event) => {
            setName(event.target.value);
          }}
        />
      </Field>
      <ConfirmPanel
        question={t('factor.passkey.hint')}
        confirm={add.Busy ? t('mfa.passkeys.adding') : t('mfa.passkeys.add')}
        icon="plus"
        methods={methods}
        action={add}
        onConfirm={() => {
          void add.Run().then((result) => {
            if (result !== null) onAdded(result);
          });
        }}
        onCancel={onCancel}
      />
    </div>
  );
};
