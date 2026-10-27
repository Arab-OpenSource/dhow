import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { apiDown, apiUp, META_BODY } from '@/Core/Testing/Fetch';
import { renderWithProviders } from '@/Core/Testing/Render';
import { Ar } from '@/I18n/Locales/Ar';
import { SystemBanners } from '@/Modules/System/Components/SystemBanners';
import { useMeta } from '@/Modules/System/Hooks/useMeta';

const MetaLoaded = () => {
  const { isSuccess } = useMeta();
  return isSuccess ? <p>meta loaded</p> : null;
};

const renderBanners = () =>
  renderWithProviders(
    <>
      <SystemBanners />
      <MetaLoaded />
    </>,
  );

describe('SystemBanners', () => {
  it('tells the user that first-time setup is not done', async () => {
    apiUp({ ...META_BODY, Data: { ...META_BODY.Data, SetupRequired: true } });
    renderBanners();
    const banner = await screen.findByRole('status');
    expect(banner).toHaveTextContent(Ar['setup.banner.title']);
    expect(banner).toHaveTextContent(Ar['setup.banner.body']);
  });

  it('stays out of the way when setup is done', async () => {
    apiUp();
    renderBanners();
    await screen.findByText('meta loaded');
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('shows an error banner when the API is down and clears it after a successful retry', async () => {
    apiDown();
    const { user } = renderBanners();
    const banner = await screen.findByRole('alert');
    expect(banner).toHaveTextContent(Ar['apiDown.title']);
    apiUp();
    await user.click(screen.getByRole('button', { name: Ar['apiDown.retry'] }));
    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
    expect(screen.getByText('meta loaded')).toBeInTheDocument();
  });
});
