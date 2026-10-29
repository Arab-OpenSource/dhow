import { useLocation } from '@tanstack/react-router';

import { matchSection, type ISectionMatch } from '@/App/Navigation/Sections';

export const useSection = (): ISectionMatch | null => {
  const pathname = useLocation({ select: (location) => location.pathname });
  return matchSection(pathname);
};
