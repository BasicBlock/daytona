/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { DAYTONA_DOCS_URL } from '@/constants/ExternalLinks'
import { useTheme } from '@/contexts/ThemeContext'
import { cn } from '@/lib/utils'
import { BookOpen, MoonIcon, SunIcon } from 'lucide-react'
import { type ComponentProps, type ReactNode, useLayoutEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { BannerStack } from './Banner'
import { Button } from './ui/button'
import { SidebarTrigger } from './ui/sidebar'

function PageLayout({ className, contained = false, ...props }: ComponentProps<'div'> & { contained?: boolean }) {
  return (
    <div
      className={cn('flex h-full flex-col group/page', { 'max-h-screen overflow-hidden': contained }, className)}
      {...props}
    />
  )
}

function PageHeaderBase({
  className,
  children,
  actions,
  ...props
}: ComponentProps<'header'> & { actions?: ReactNode }) {
  return (
    <header
      className={cn(
        'flex gap-2 sm:gap-4 items-center border-b border-border px-4 py-[15px] bg-background z-10 min-h-[55px]',
        className,
      )}
      {...props}
    >
      <SidebarTrigger className="shrink-0 [&_svg]:size-5 md:hidden" />
      <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-4">{children}</div>
      {actions ? <div className="flex shrink-0 items-center">{actions}</div> : null}
    </header>
  )
}

function PageHeader(props: ComponentProps<'header'>) {
  return (
    <PageHeaderBase
      {...props}
      actions={
        <>
          <PageHeaderExternalAction
            label="Docs"
            href={DAYTONA_DOCS_URL}
            icon={<BookOpen className="size-4" />}
            variant="link"
          />
          <PageHeaderThemeAction />
        </>
      }
    />
  )
}

function PageHeaderExternalAction({
  label,
  className,
  href,
  icon,
  variant = 'ghost',
}: {
  label: string
  href: string
  icon: ReactNode
  variant?: 'ghost' | 'link'
  className?: string
}) {
  return (
    <Button variant={variant} size="sm" className={cn('text-muted-foreground', className)} aria-label={label} asChild>
      <a href={href} target="_blank" rel="noopener noreferrer">
        {icon}
        <span className="hidden md:inline">{label}</span>
      </a>
    </Button>
  )
}

function PageHeaderThemeAction() {
  const { theme, setTheme } = useTheme()

  return (
    <Button
      type="button"
      variant="link"
      size="sm"
      className="text-muted-foreground hover:text-foreground"
      aria-label="Toggle theme"
      onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
    >
      {theme === 'dark' ? <SunIcon className="size-4" /> : <MoonIcon className="size-4" />}
      <span className="hidden md:inline">{theme === 'dark' ? 'Light' : 'Dark'}</span>
    </Button>
  )
}

function PageTitle({ className, children, ...props }: ComponentProps<'h1'>) {
  return (
    <h1 className={cn('text-2xl sm:text-3.5xl font-medium tracking-tight', className)} {...props}>
      {children}
    </h1>
  )
}

function PageIntro({
  title,
  desc,
  actions,
  className,
}: {
  title: ReactNode
  desc?: ReactNode
  actions?: ReactNode
  className?: string
}) {
  return (
    <div className={cn('mb-8 shrink-0 flex flex-col gap-1', className)}>
      <div className="flex min-w-0 flex-wrap items-start justify-between gap-x-4 gap-y-3">
        <div className="flex flex-1 flex-col gap-1">
          <PageTitle>{title}</PageTitle>
          {desc ? <div className="text-sm text-muted-foreground">{desc}</div> : null}
        </div>
        {actions ? (
          <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-x-1 gap-y-2">{actions}</div>
        ) : null}
      </div>
    </div>
  )
}

function PageBanner({ className, children, ...props }: ComponentProps<'div'>) {
  return (
    <div data-slot="page-banner" className={cn('w-full relative z-30 empty:hidden', className)} {...props}>
      {children}
    </div>
  )
}

function PageContent({
  className,
  size = 'default',
  children,
  ...props
}: ComponentProps<'main'> & { size?: 'default' | 'full' }) {
  return (
    <main
      className={cn(
        'flex flex-col gap-4 p-4 w-full flex-1 min-h-0 overflow-auto',
        {
          'max-w-5xl mx-auto': size === 'default',
        },
        className,
      )}
      {...props}
    >
      <PageBanner>
        <BannerStack bannerClassName={cn({ 'max-w-5xl mx-auto': size === 'default' })} />
      </PageBanner>
      {children}
    </main>
  )
}

function PageFooterPortal({ children }: { children: ReactNode }): ReactNode {
  const [container, setContainer] = useState<Element | null>(null)

  useLayoutEffect(() => {
    setContainer(document.querySelector('[data-slot="page-footer"]'))
  }, [])

  if (!container) return children

  return <>{createPortal(children, container)}</>
}

function PageFooter({ className, children, ...props }: ComponentProps<'footer'>) {
  return (
    <footer
      data-slot="page-footer"
      className={cn(
        'flex gap-2 sm:gap-4 items-center border-t border-border p-4 bg-background z-10 empty:hidden',
        className,
      )}
      {...props}
    >
      {children}
    </footer>
  )
}

export { PageContent, PageFooter, PageFooterPortal, PageHeader, PageHeaderBase, PageIntro, PageLayout, PageTitle }
