import { useState } from 'react';

import { useThemeStore, type IThemeDraft } from '@/Core/Theme/useThemeStore';

export const useLastDraft = (): IThemeDraft | null => {
  const Draft = useThemeStore((state) => state.Draft);
  const [Last, setLast] = useState<IThemeDraft | null>(Draft);

  if (Draft && Draft !== Last) setLast(Draft);

  return Draft ?? Last;
};
