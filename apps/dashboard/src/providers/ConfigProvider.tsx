import { queryKeys } from '@/hooks/queries/queryKeys'
import { DaytonaConfiguration } from '@daytona/api-client'
import { useSuspenseQuery } from '@tanstack/react-query'
import { ReactNode } from 'react'
import { ConfigContext } from '../contexts/ConfigContext'

const apiUrl = import.meta.env.VITE_API_URL ?? window.location.origin + '/api'

type Props = {
  children: ReactNode
}

export function ConfigProvider(props: Props) {
  const { data: config } = useSuspenseQuery({
    queryKey: queryKeys.config.all,
    queryFn: async () => {
      const res = await fetch(`${apiUrl}/config`)
      if (!res.ok) {
        throw res
      }
      return res.json() as Promise<DaytonaConfiguration>
    },
  })

  return <ConfigContext.Provider value={{ ...config, apiUrl }}>{props.children}</ConfigContext.Provider>
}
