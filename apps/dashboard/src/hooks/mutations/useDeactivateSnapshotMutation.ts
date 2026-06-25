/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../queries/queryKeys'
import { useApi } from '../useApi'

export interface DeactivateSnapshotMutationVariables {
  snapshotId: string
}

interface UseDeactivateSnapshotMutationOptions {
  invalidateOnSuccess?: boolean
}

export const useDeactivateSnapshotMutation = ({
  invalidateOnSuccess = true,
}: UseDeactivateSnapshotMutationOptions = {}) => {
  const { snapshotApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation<void, unknown, DeactivateSnapshotMutationVariables>({
    mutationFn: async ({ snapshotId }) => {
      await snapshotApi.deactivateSnapshot(snapshotId)
    },
    onSuccess: async () => {
      if (invalidateOnSuccess) {
        await queryClient.invalidateQueries({
          queryKey: queryKeys.snapshots.list(),
        })
      }
    },
  })
}
