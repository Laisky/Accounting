import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createAccount,
  createAccountGroup,
  fetchAccountGroups,
  fetchAccounts,
  fetchExchangeRates,
  fetchLedgerSummary,
  updateAccountGroup,
  type Account,
  type AccountGroup,
  type AccountGroupUpdateInput,
} from '@/lib/api/ledger';
import type { components } from '@/lib/api/generated/schema';
import { queryKeys } from './queryKeys';

type AccountCreateRequest = components['schemas']['AccountCreateRequest'];

// useAccountsQuery loads the actor-owned accounts through the shared Query cache.
export function useAccountsQuery() {
  return useQuery({
    queryFn: fetchAccounts,
    queryKey: queryKeys.accounts.list(),
  });
}

// useAccountGroupsQuery loads the actor-owned account groups through the shared Query cache.
export function useAccountGroupsQuery() {
  return useQuery({
    queryFn: fetchAccountGroups,
    queryKey: queryKeys.accounts.groups(),
  });
}

// useExchangeRatesQuery loads USD-relative exchange rates through the shared Query cache.
export function useExchangeRatesQuery() {
  return useQuery({
    queryFn: fetchExchangeRates,
    queryKey: queryKeys.exchangeRates.list(),
    staleTime: 5 * 60_000,
  });
}

// useLedgerSummaryQuery loads the cross-book balance summary through the shared Query cache.
export function useLedgerSummaryQuery() {
  return useQuery({
    queryFn: ({ signal }) => fetchLedgerSummary(signal),
    queryKey: queryKeys.ledger.summary(),
  });
}

// useCreateAccountMutation creates an account and refreshes accounts plus balance summary.
export function useCreateAccountMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (input: AccountCreateRequest): Promise<Account> => createAccount(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['accounts', 'list'] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.ledger.summary() });
      void queryClient.invalidateQueries({ queryKey: ['reports'] });
    },
  });
}

// useCreateAccountGroupMutation creates an account group and refreshes the group list.
export function useCreateAccountGroupMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string): Promise<AccountGroup> => createAccountGroup(name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['accounts', 'groups'] });
    },
  });
}

// useUpdateAccountGroupMutation patches an account group and refreshes the group list.
export function useUpdateAccountGroupMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ groupId, input }: { groupId: string; input: AccountGroupUpdateInput }): Promise<AccountGroup> =>
      updateAccountGroup(groupId, input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['accounts', 'groups'] });
    },
  });
}
