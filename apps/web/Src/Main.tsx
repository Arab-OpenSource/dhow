import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { App } from '@/App/App';
import '@/Styles/Fonts.css';
import '@/Styles/Globals.css';

const container = document.getElementById('root');
if (!container) throw new Error('root element is missing');

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
