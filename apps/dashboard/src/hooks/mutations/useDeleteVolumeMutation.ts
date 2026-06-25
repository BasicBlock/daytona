/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../queries/queryKeys'
import { useApi } from '../useApi'

export interface DeleteVolumeMutationVariables {
  volumeId: string
}

interface UseDeleteVolumeMutationOptions {
  invalidateOnSuccess?: boolean
}

export const useDeleteVolumeMutation = ({ invalidateOnSuccess = true }: UseDeleteVolumeMutationOptions = {}) => {
  const { volumeApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation<void, unknown, DeleteVolumeMutationVariables>({
    mutationFn: async ({ volumeId }) => {
      await volumeApi.deleteVolume(volumeId)
    },
    onSuccess: async () => {
      if (invalidateOnSuccess) {
        await queryClient.invalidateQueries({ queryKey: queryKeys.volumes.list() })
      }
    },
  })
}
