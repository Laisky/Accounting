import '@testing-library/jest-dom/vitest';
import { configure } from '@testing-library/react';

// Cold coverage transforms of lazy routes can exceed Testing Library's one-second
// default on bounded CI workers. Keep the wait within Vitest's test budget while
// preserving the same accessible-role and data assertions.
configure({ asyncUtilTimeout: 5000 });
// Initialize i18next (English bundled synchronously) so components using
// useTranslation render translated copy during tests.
import '../i18n';
