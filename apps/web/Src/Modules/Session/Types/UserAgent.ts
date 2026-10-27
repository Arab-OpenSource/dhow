export interface IDevice {
  Browser: string | null;
  Os: string | null;
  Mobile: boolean;
}

const BROWSERS: readonly (readonly [RegExp, string])[] = [
  [/Edg(e|A|iOS)?\//, 'Edge'],
  [/OPR\/|Opera/, 'Opera'],
  [/Firefox\/|FxiOS\//, 'Firefox'],
  [/Chrome\/|CriOS\//, 'Chrome'],
  [/Safari\//, 'Safari'],
];

const SYSTEMS: readonly (readonly [RegExp, string])[] = [
  [/Windows/, 'Windows'],
  [/Android/, 'Android'],
  [/iPhone|iPad|iPod/, 'iOS'],
  [/Mac OS X|Macintosh/, 'macOS'],
  [/CrOS/, 'ChromeOS'],
  [/Linux|X11/, 'Linux'],
];

const MOBILE = /Android|iPhone|iPad|iPod|Mobile/;

const firstMatch = (table: readonly (readonly [RegExp, string])[], userAgent: string): string | null =>
  table.find(([pattern]) => pattern.test(userAgent))?.[1] ?? null;

export const describeUserAgent = (userAgent: string): IDevice => ({
  Browser: firstMatch(BROWSERS, userAgent),
  Os: firstMatch(SYSTEMS, userAgent),
  Mobile: MOBILE.test(userAgent),
});
