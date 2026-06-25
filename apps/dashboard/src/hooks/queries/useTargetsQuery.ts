/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Target, TargetType } from '@daytona/api-client'
import { useQuery, UseQueryOptions } from '@tanstack/react-query'
import { useMemo } from 'react'
import { createTargetNameGetter } from '@/lib/targets'
import { queryKeys } from './queryKeys'

const PUBLIC_TARGETS: Target[] = [
  {
    id: 'default',
    name: 'Default',
    targetType: TargetType.CUSTOM,
  },
]

type TargetsQueryOptions = Omit<UseQueryOptions<Target[]>, 'queryKey' | 'queryFn' | 'enabled'> & {
  enabled?: boolean
}

export function useSharedTargetsQuery(options?: TargetsQueryOptions) {
  const { enabled = true, ...queryOptions } = options ?? {}

  return useQuery<Target[]>({
    queryKey: queryKeys.targets.shared(),
    queryFn: async () => PUBLIC_TARGETS,
    enabled,
    ...queryOptions,
  })
}

export function useAvailableTargetsQuery(options?: TargetsQueryOptions) {
  const { enabled = true, ...queryOptions } = options ?? {}

  return useQuery<Target[]>({
    queryKey: queryKeys.targets.available(),
    queryFn: async () => PUBLIC_TARGETS,
    enabled,
    ...queryOptions,
  })
}

export function useTargetLookup() {
  const availableTargetsQuery = useAvailableTargetsQuery()
  const sharedTargetsQuery = useSharedTargetsQuery()

  const getKnownTargetName = useMemo(
    () => createTargetNameGetter(availableTargetsQuery.data ?? [], sharedTargetsQuery.data ?? []),
    [availableTargetsQuery.data, sharedTargetsQuery.data],
  )

  const getTargetName = useMemo(
    (): ((target: string) => string) => (target: string) => getKnownTargetName(target) ?? target,
    [getKnownTargetName],
  )

  return useMemo(
    () => ({
      getTargetName,
      isLoading: availableTargetsQuery.isLoading || sharedTargetsQuery.isLoading,
      isFetching: availableTargetsQuery.isFetching || sharedTargetsQuery.isFetching,
    }),
    [
      availableTargetsQuery.isFetching,
      availableTargetsQuery.isLoading,
      getTargetName,
      sharedTargetsQuery.isFetching,
      sharedTargetsQuery.isLoading,
    ],
  )
}
