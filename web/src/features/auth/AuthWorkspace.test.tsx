import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { AuthWorkspace } from './AuthWorkspace';
import { emptyRuntimeConfig } from '../../lib/api/runtimeConfig';

describe('AuthWorkspace', () => {
  it('renders external SSO when runtime config enables it', () => {
    render(
      <AuthWorkspace
        runtimeConfig={{
          ...emptyRuntimeConfig,
          features: {
            ...emptyRuntimeConfig.features,
            externalSsoEnabled: true,
          },
          sso: {
            enabled: true,
            startPath: '/api/auth/sso/start',
          },
        }}
        onAuthenticated={vi.fn()}
      />,
    );

    expect(screen.getByText('External SSO')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Use SSO' })).toHaveAttribute('href', '/api/auth/sso/start');
  });

  it('hides external SSO when runtime config disables it', () => {
    render(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={vi.fn()} />);

    expect(screen.queryByText('External SSO')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Use SSO' })).not.toBeInTheDocument();
  });
});
