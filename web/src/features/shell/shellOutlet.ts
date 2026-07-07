import { useOutletContext } from 'react-router';

// ShellOutletContext carries shell-owned UI state down to routed views without prop drilling.
export type ShellOutletContext = {
  entryEditorOpenSignal: number;
  setProcessing: (label: string) => void;
};

// useShellOutlet returns the shell-owned UI signals for a routed view.
export function useShellOutlet(): ShellOutletContext {
  return useOutletContext<ShellOutletContext>();
}
