/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../queries/queryKeys'
import { useApi } from '../useApi'

export interface DeleteSnapshotMutationVariables {
  snapshotId: string
}

interface UseDeleteSnapshotMutationOptions {
  invalidateOnSuccess?: boolean
}

export const useDeleteSnapshotMutation = ({ invalidateOnSuccess = true }: UseDeleteSnapshotMutationOptions = {}) => {
  const { snapshotApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation<void, unknown, DeleteSnapshotMutationVariables>({
    mutationFn: async ({ snapshotId }) => {
      await snapshotApi.removeSnapshot(snapshotId)
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
