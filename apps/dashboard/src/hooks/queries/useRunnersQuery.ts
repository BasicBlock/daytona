/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Runner } from '@daytona/api-client'
import { useQuery, UseQueryOptions } from '@tanstack/react-query'
import { useApi } from '../useApi'
import { queryKeys } from './queryKeys'

type RunnersQueryOptions = Omit<UseQueryOptions<Runner[]>, 'queryKey' | 'queryFn' | 'enabled'> & {
  enabled?: boolean
  target?: string
}

export function useRunnersQuery(options?: RunnersQueryOptions) {
  const { runnersApi } = useApi()
  const { enabled = true, target, ...queryOptions } = options ?? {}
  const normalizedTarget = target || undefined

  return useQuery<Runner[]>({
    queryKey: queryKeys.runners.list(normalizedTarget),
    meta: {
      errorMessage: 'Failed to fetch runners',
    },
    queryFn: async () => {
      const response = await runnersApi.listRunners(normalizedTarget)
      return response.data ?? []
    },
    enabled,
    staleTime: 1000 * 10,
    gcTime: 1000 * 60 * 5,
    ...queryOptions,
  })
}
