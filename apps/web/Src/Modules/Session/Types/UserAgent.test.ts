import { describe, expect, it } from 'vitest';

import { CHROME_UA, FIREFOX_UA, SAFARI_UA } from '@/Core/Testing/Backend';
import { describeUserAgent } from '@/Modules/Session/Types/UserAgent';

describe('describeUserAgent', () => {
  it.each([
    [CHROME_UA, 'Chrome', 'Linux', false],
    [FIREFOX_UA, 'Firefox', 'Windows', false],
    [SAFARI_UA, 'Safari', 'iOS', true],
    ['Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Chrome/140.0 Safari/537.36 Edg/140.0', 'Edge', 'Windows', false],
    ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Version/18 Safari/605.1.15', 'Safari', 'macOS', false],
    ['Mozilla/5.0 (Linux; Android 15; Pixel) AppleWebKit/537.36 Chrome/140.0 Mobile Safari/537.36', 'Chrome', 'Android', true],
    ['curl/8.5.0', null, null, false],
    ['', null, null, false],
  ])('reads %s', (userAgent, browser, os, mobile) => {
    expect(describeUserAgent(userAgent)).toEqual({ Browser: browser, Os: os, Mobile: mobile });
  });
});
