/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { VolumeDto } from '@daytona/api-client'
import { useQuery } from '@tanstack/react-query'
import { useApi } from '../useApi'
import { queryKeys } from './queryKeys'

export function useVolumesQuery() {
  const { volumeApi } = useApi()

  return useQuery<VolumeDto[]>({
    queryKey: queryKeys.volumes.list(),
    queryFn: async () => {
      const response = await volumeApi.listVolumes()
      return response.data
    },
  })
}
