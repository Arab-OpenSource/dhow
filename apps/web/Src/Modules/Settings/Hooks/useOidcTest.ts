import { useState } from 'react';

import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { testOidcProvider } from '@/Modules/Settings/Api/OidcApi';
import type { TOidcTest, TOidcType } from '@/Modules/Settings/Types/OidcTypes';

export interface IOidcTestState {
  Busy: boolean;
  Result: TOidcTest | null;
  Problem: string | null;
  Run: (type: TOidcType, issuer: string) => void;
  Reset: () => void;
}

export const useOidcTest = (): IOidcTestState => {
  const errorText = useApiErrorText();
  const [Busy, setBusy] = useState(false);
  const [Result, setResult] = useState<TOidcTest | null>(null);
  const [Problem, setProblem] = useState<string | null>(null);

  const Run = (type: TOidcType, issuer: string) => {
    setBusy(true);
    setResult(null);
    setProblem(null);
    testOidcProvider(type, issuer)
      .then(setResult)
      .catch((error: unknown) => {
        setProblem(errorText(error));
      })
      .finally(() => {
        setBusy(false);
      });
  };

  const Reset = () => {
    setResult(null);
    setProblem(null);
  };

  return { Busy, Result, Problem, Run, Reset };
};
