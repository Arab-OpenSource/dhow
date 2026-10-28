import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { App } from '@/App/App';
import { takeInviteTokenFromLocation } from '@/Modules/Identity/Store/InviteToken';
import { takeSetupTokenFromLocation } from '@/Modules/Identity/Store/SetupToken';
import '@/Styles/Fonts.css';
import '@/Styles/Globals.css';

takeSetupTokenFromLocation();
takeInviteTokenFromLocation();

const container = document.getElementById('root');
if (!container) throw new Error('root element is missing');

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
