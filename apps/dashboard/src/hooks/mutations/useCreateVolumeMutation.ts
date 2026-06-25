/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { CreateVolume, VolumeDto } from '@daytona/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../queries/queryKeys'
import { useApi } from '../useApi'

export interface CreateVolumeMutationVariables {
  volume: CreateVolume
}

export const useCreateVolumeMutation = () => {
  const { volumeApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation<VolumeDto, unknown, CreateVolumeMutationVariables>({
    mutationFn: async ({ volume }) => {
      const response = await volumeApi.createVolume(volume)
      return response.data
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.volumes.list() })
    },
  })
}
