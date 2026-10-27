import { useCallback, useState } from 'react';

import { API_ERROR_NETWORK, ApiError, isApiError } from '@/Core/Api/ApiError';

export interface IAction<TArgs extends unknown[], TResult> {
  Run: (...args: TArgs) => Promise<TResult | ApiError>;
  Pending: boolean;
  Failure: ApiError | null;
  Reset: () => void;
}

const asApiError = (error: unknown): ApiError =>
  isApiError(error) ? error : new ApiError({ Code: API_ERROR_NETWORK, Message: '' }, 0);

export const useAction = <TArgs extends unknown[], TResult>(
  action: (...args: TArgs) => Promise<TResult>,
): IAction<TArgs, TResult> => {
  const [Pending, setPending] = useState(false);
  const [Failure, setFailure] = useState<ApiError | null>(null);

  const Run = useCallback(
    async (...args: TArgs): Promise<TResult | ApiError> => {
      setPending(true);
      setFailure(null);
      try {
        return await action(...args);
      } catch (error) {
        const failure = asApiError(error);
        setFailure(failure);
        return failure;
      } finally {
        setPending(false);
      }
    },
    [action],
  );

  const Reset = useCallback(() => {
    setFailure(null);
  }, []);

  return { Run, Pending, Failure, Reset };
};
