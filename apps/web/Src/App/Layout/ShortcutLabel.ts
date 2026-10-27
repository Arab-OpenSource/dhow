const APPLE_PLATFORM = /Mac|iPhone|iPad|iPod/;

export const shortcutLabel = (): string => (APPLE_PLATFORM.test(navigator.userAgent) ? '⌘K' : 'Ctrl K');
