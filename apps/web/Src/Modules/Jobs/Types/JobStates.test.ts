import { describe, expect, it } from 'vitest';

import { JOBS_GUIDE_URL } from '@/Modules/Jobs/Types/JobStates';

describe('the jobs guide link', () => {
  it('opens the Arabic guide in Arabic and the English mirror in English, each at its own heading', () => {
    expect(JOBS_GUIDE_URL.ar).toBe('https://github.com/Khaymah/dhow/blob/main/docs/guides/jobs.md#إنشاء-تشغيل-عبر-api');
    expect(JOBS_GUIDE_URL.en).toBe('https://github.com/Khaymah/dhow/blob/main/docs/en/guides/jobs.md#create-a-job-through-the-api');
  });
});
