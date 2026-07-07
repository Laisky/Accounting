import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { fetchUserProfile, updateUserProfile, type AuthUser } from '@/lib/api/auth';
import { queryKeys } from './queryKeys';

// useUserProfileQuery loads the signed-in user's profile (including base currency) through Query.
export function useUserProfileQuery() {
  return useQuery({
    queryFn: ({ signal }) => fetchUserProfile(signal),
    queryKey: queryKeys.user.me(),
  });
}

// useUpdateUserProfileMutation patches the user profile and refreshes the cached profile.
export function useUpdateUserProfileMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (input: { baseCurrency?: string }): Promise<AuthUser> => updateUserProfile(input),
    onSuccess: (user) => {
      queryClient.setQueryData(queryKeys.user.me(), user);
    },
  });
}
