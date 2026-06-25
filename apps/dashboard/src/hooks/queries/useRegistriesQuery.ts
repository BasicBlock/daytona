/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { DockerRegistry } from '@daytona/api-client'
import { useQuery } from '@tanstack/react-query'
import { useApi } from '../useApi'
import { queryKeys } from './queryKeys'

export function useRegistriesQuery() {
  const { dockerRegistryApi } = useApi()

  return useQuery<DockerRegistry[]>({
    queryKey: queryKeys.registries.list(),
    queryFn: async () => {
      const response = await dockerRegistryApi.listRegistries()
      return response.data
    },
  })
}
