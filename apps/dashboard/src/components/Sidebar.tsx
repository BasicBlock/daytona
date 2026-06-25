/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import {
  Sidebar as SidebarComponent,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
  SidebarTrigger,
  useSidebar,
} from '@/components/ui/sidebar'
import { RoutePath } from '@/enums/RoutePath'
import { useCommandPaletteAnalytics } from '@/hooks/useCommandPaletteAnalytics'
import { cn, getMetaKey } from '@/lib/utils'
import { lazyRoutes } from '@/routes'
import { ArrowRightIcon, Box, Container, HardDrive, PackageOpen, SearchIcon, Server } from 'lucide-react'
import { AnimatePresence } from 'motion/react'
import React, { useMemo } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import { AnimatedLogo } from './AnimatedLogo'
import { CommandConfig, useCommandPaletteActions, useRegisterCommands } from './CommandPalette'
import { Kbd } from './ui/kbd'
import { ScrollArea } from './ui/scroll-area'
import { Separator } from './ui/separator'

interface SidebarProps {
  isBannerVisible: boolean
  version: string
}

interface SidebarItem {
  icon: React.ReactElement
  label: string
  path: RoutePath
  preload?: () => Promise<unknown>
}

function preloadSidebarItem(item: Pick<SidebarItem, 'preload'>) {
  item.preload?.().catch(() => {
    // React Router will surface import failures when the route renders.
  })
}

const useNavCommands = (items: SidebarItem[]) => {
  const { pathname } = useLocation()
  const navigate = useNavigate()

  const navCommands: CommandConfig[] = useMemo(
    () =>
      items
        .filter((item) => item.path !== pathname)
        .map((item) => ({
          id: `nav-${item.path}`,
          label: `Go to ${item.label}`,
          icon: <ArrowRightIcon className="w-4 h-4" />,
          onSelect: () => {
            preloadSidebarItem(item)
            navigate(item.path)
          },
        })),
    [pathname, navigate, items],
  )

  useRegisterCommands(navCommands, { groupId: 'navigation', groupLabel: 'Navigation', groupOrder: 1 })
}

export function Sidebar({ isBannerVisible, version }: SidebarProps) {
  const { pathname, search } = useLocation()
  const sidebar = useSidebar()
  const { isMobile, setOpenMobile } = sidebar

  const sidebarItems = useMemo<SidebarItem[]>(
    () => [
      {
        icon: <Container size={16} strokeWidth={1.5} />,
        label: 'Sandboxes',
        path: RoutePath.SANDBOXES,
        preload: lazyRoutes.Sandboxes,
      },
      {
        icon: <Box size={16} strokeWidth={1.5} />,
        label: 'Snapshots',
        path: RoutePath.SNAPSHOTS,
        preload: lazyRoutes.Snapshots,
      },
      {
        icon: <PackageOpen size={16} strokeWidth={1.5} />,
        label: 'Registries',
        path: RoutePath.REGISTRIES,
        preload: lazyRoutes.Registries,
      },
      {
        icon: <HardDrive size={16} strokeWidth={1.5} />,
        label: 'Volumes',
        path: RoutePath.VOLUMES,
        preload: lazyRoutes.Volumes,
      },
      {
        icon: <Server size={16} strokeWidth={1.5} />,
        label: 'Runners',
        path: RoutePath.RUNNERS,
        preload: lazyRoutes.Runners,
      },
    ],
    [],
  )

  useNavCommands(sidebarItems)

  const commandPaletteActions = useCommandPaletteActions()
  const { trackOpened } = useCommandPaletteAnalytics()
  const metaKey = getMetaKey()

  React.useEffect(() => {
    if (isMobile) {
      setOpenMobile(false)
    }
  }, [isMobile, pathname, search, setOpenMobile])

  const sidebarExpanded = sidebar.open || sidebar.openMobile

  return (
    <SidebarComponent isBannerVisible={isBannerVisible} collapsible="icon">
      <SidebarHeader>
        <div
          className={cn('flex h-[46px] items-center justify-between gap-2 px-2 pt-2', {
            'justify-center px-0': !sidebarExpanded,
          })}
        >
          <div className="flex items-center gap-2 group-data-[state=collapsed]:hidden text-primary">
            <AnimatePresence initial={false}>
              {sidebarExpanded && <AnimatedLogo className={cn('w-[117px]')} key={String(sidebar.open)} />}
            </AnimatePresence>
          </div>
          <div className="relative">
            <SidebarTrigger className={cn('p-2 [&_svg]:size-5 transition-all peer')} />
          </div>
        </div>
      </SidebarHeader>
      <Separator className="mx-0 w-full" />
      <SidebarContent className="pt-4">
        <SidebarMenu className="px-2 pb-2 gap-2">
          <SidebarMenuItem>
            <SidebarMenuButton
              tooltip={`Search ${metaKey}+K`}
              variant="outline"
              className="justify-between bg-input/50"
              onClick={() => {
                trackOpened('sidebar_search')
                commandPaletteActions.setIsOpen(true)
              }}
            >
              <span className="flex min-w-0 items-center gap-2">
                <SearchIcon className="size-4" />
                <span className="truncate group-data-[collapsible=icon]:hidden">Search</span>
              </span>
              <Kbd className="ml-auto whitespace-nowrap group-data-[collapsible=icon]:hidden">{metaKey} K</Kbd>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
        <ScrollArea fade="mask" className="overflow-auto flex-1">
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                {sidebarItems.map((item) => (
                  <SidebarMenuItem key={item.label}>
                    <SidebarMenuButton
                      asChild
                      isActive={pathname.startsWith(item.path)}
                      className="text-sm"
                      tooltip={item.label}
                    >
                      <Link
                        to={item.path}
                        onPointerEnter={() => preloadSidebarItem(item)}
                        onFocus={() => preloadSidebarItem(item)}
                      >
                        {item.icon}
                        <span>{item.label}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
          <SidebarSeparator />
        </ScrollArea>
      </SidebarContent>
      <SidebarFooter className="pb-4">
        <div className="px-2 text-left text-xs text-muted-foreground group-data-[collapsible=icon]:hidden overflow-hidden whitespace-nowrap">
          Version {version}
        </div>
      </SidebarFooter>
    </SidebarComponent>
  )
}
