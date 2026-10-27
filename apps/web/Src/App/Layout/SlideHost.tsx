import { useEffect, useEffectEvent, useState } from 'react';

import { buildSlide } from '@/App/Layout/FrameSlides';
import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { isThemeEditFrame, themeOf } from '@/App/Panels';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useThemeStore } from '@/Core/Theme/useThemeStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { SlideStack } from '@/Core/Ui/SlideStack';
import { useT } from '@/I18n/useT';

export const SlideHost = () => {
  const t = useT();
  const navigation = useShellNavigation();
  const role = useCurrentMembership()?.Role;
  const draftName = useThemeStore((state) => state.Draft?.Name);
  const customThemes = useThemeStore((state) => state.CustomThemes);
  const DiscardDraft = useThemeStore((state) => state.DiscardDraft);
  const Push = useToastStore((state) => state.Push);
  const { stack, invalidStack } = navigation;
  const editing = stack.some(isThemeEditFrame);
  const [missingOnLoad] = useState(() =>
    stack.some((id) => {
      const themeId = themeOf(id);
      return themeId !== null && !customThemes.some((theme) => theme.Id === themeId);
    }),
  );

  const rejectLink = useEffectEvent(() => {
    Push({
      Id: 'unknown-panel',
      Accent: 'awaiting',
      Title: t('toast.unknownPanel.title'),
      Detail: t('toast.unknownPanel.detail'),
    });
    navigation.discardStack();
  });

  useEffect(() => {
    if (!editing) DiscardDraft();
  }, [editing, DiscardDraft]);

  useEffect(() => {
    if (invalidStack || missingOnLoad) rejectLink();
  }, [invalidStack, missingOnLoad]);

  const slides = stack.map((id) => buildSlide(id, { t, navigation, draftName, customThemes, role }));

  return (
    <SlideStack
      slides={slides}
      onPop={navigation.popFrame}
      onCloseAll={navigation.closeAll}
      onJump={(index, source) => {
        navigation.jumpTo(index);
        if (source === 'slide') Push({ Id: 'slides-returned', Accent: 'live', Title: t('slides.returned.title') });
      }}
    />
  );
};
