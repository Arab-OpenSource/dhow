import { isMessageKey } from '@/I18n/Translate';
import { useT } from '@/I18n/useT';

export const useFormMessage = (): ((message: string | undefined) => string | undefined) => {
  const t = useT();
  return (message) => {
    if (message === undefined) return undefined;
    return isMessageKey(message) ? t(message) : message;
  };
};
