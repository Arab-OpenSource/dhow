import { describe, expect, it } from 'vitest';

import { registerCommandFor } from '@/Modules/Runners/Types/RunnerView';

describe('registerCommandFor', () => {
  it.each([
    ['https://dhow.example.com', 'sudo -u dhow-runner dhow-runner register --url https://dhow.example.com --token-file -'],
    ['https://localhost:8443', 'sudo -u dhow-runner dhow-runner register --url https://localhost:8443 --token-file -'],
    ['http://dhow.internal:8080', 'sudo -u dhow-runner dhow-runner register --url http://dhow.internal:8080 --token-file -'],
    ['http://localhost:8080', 'sudo -u dhow-runner dhow-runner register --dev --url http://localhost:8080 --token-file -'],
    ['http://127.0.0.1:58120', 'sudo -u dhow-runner dhow-runner register --dev --url http://127.0.0.1:58120 --token-file -'],
    ['http://[::1]:8080', 'sudo -u dhow-runner dhow-runner register --dev --url http://[::1]:8080 --token-file -'],
  ])('builds the documented command for %s', (origin, command) => {
    expect(registerCommandFor(origin)).toBe(command);
  });
});
