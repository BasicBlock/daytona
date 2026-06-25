/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import React, { Ref, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react'
import { Target, CreateRunnerResponse } from '@daytona/api-client'
import { useForm } from '@tanstack/react-form'
import { z } from 'zod'
import { InfoIcon } from 'lucide-react'
import { CreateResourceButton } from '@/components/CreateResourceButton'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Spinner } from '@/components/ui/spinner'
import { useCreateRunnerMutation } from '@/hooks/mutations/useCreateRunnerMutation'
import { handleApiError } from '@/lib/error-handling'
import { toast } from 'sonner'

const formSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  target: z.string().min(1, 'Target is required'),
})

type FormValues = z.infer<typeof formSchema>

interface CreateRunnerSheetProps {
  targets: Target[]
  ref?: Ref<{ open: () => void }>
}

const buildDefaultValues = (targets: Target[]): FormValues => ({
  name: '',
  target: targets[0]?.id ?? '',
})

export const CreateRunnerSheet: React.FC<CreateRunnerSheetProps> = ({ targets, ref }) => {
  const [open, setOpen] = useState(false)
  const [createdRunner, setCreatedRunner] = useState<CreateRunnerResponse | null>(null)
  const formRef = useRef<HTMLFormElement>(null)
  const { reset: resetCreateRunnerMutation, ...createRunnerMutation } = useCreateRunnerMutation()

  useImperativeHandle(ref, () => ({
    open: () => setOpen(true),
  }))

  const defaultValues = useMemo(() => buildDefaultValues(targets), [targets])

  const form = useForm({
    defaultValues,
    validators: {
      onSubmit: formSchema,
    },
    onSubmitInvalid: () => {
      const formEl = formRef.current
      if (!formEl) return
      const invalidInput = formEl.querySelector('[aria-invalid="true"]') as HTMLElement | null
      if (invalidInput) {
        invalidInput.scrollIntoView({ behavior: 'smooth', block: 'center' })
        invalidInput.focus()
      }
    },
    onSubmit: async ({ value }) => {
      try {
        const runner = await createRunnerMutation.mutateAsync({
          runner: {
            name: value.name.trim(),
            target: value.target,
          },
        })

        toast.success('Runner created successfully')
        setCreatedRunner(runner)
        resetForm(buildDefaultValues(targets))
      } catch (error) {
        handleApiError(error, 'Failed to create runner')
      }
    },
  })
  const { reset: resetForm } = form

  useEffect(() => {
    if (!form.getFieldValue('target') && targets[0]?.id) {
      form.setFieldValue('target', targets[0].id)
    }
  }, [form, targets])

  const resetState = useCallback(() => {
    setCreatedRunner(null)
    resetForm(buildDefaultValues(targets))
    resetCreateRunnerMutation()
  }, [resetForm, resetCreateRunnerMutation, targets])

  useEffect(() => {
    if (open) {
      resetState()
    }
  }, [open, resetState])

  if (targets.length === 0) {
    return null
  }

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <CreateResourceButton resource="Runner" />
      </SheetTrigger>

      <SheetContent className="w-dvw sm:w-[500px] p-0 flex flex-col gap-0">
        <SheetHeader className="border-b border-border p-4 px-5 items-center flex text-left flex-row">
          <SheetTitle>{createdRunner ? 'Runner Created' : 'Create Runner'}</SheetTitle>
          <SheetDescription className="sr-only">
            {createdRunner
              ? 'Your runner has been created successfully.'
              : 'Add configuration for a new runner in your selected target.'}
          </SheetDescription>
        </SheetHeader>

        <ScrollArea fade="mask" className="flex-1 min-h-0">
          <div className="p-5">
            {createdRunner ? (
              <CreatedRunnerDisplay createdRunner={createdRunner} />
            ) : (
              <form
                ref={formRef}
                id="create-runner-form"
                className="space-y-6"
                onSubmit={(e) => {
                  e.preventDefault()
                  e.stopPropagation()
                  form.handleSubmit()
                }}
              >
                <form.Field name="target">
                  {(field) => {
                    const isInvalid = field.state.meta.isTouched && !field.state.meta.isValid
                    return (
                      <Field data-invalid={isInvalid}>
                        <FieldLabel htmlFor={field.name}>Target</FieldLabel>
                        <Select
                          value={field.state.value}
                          onValueChange={(value) => {
                            field.handleChange(value)
                          }}
                        >
                          <SelectTrigger className="h-8" id={field.name} aria-invalid={isInvalid}>
                            <SelectValue placeholder="Select a target" />
                          </SelectTrigger>
                          <SelectContent>
                            {targets.map((target) => (
                              <SelectItem key={target.id} value={target.id}>
                                {target.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        {field.state.meta.errors.length > 0 && field.state.meta.isTouched && (
                          <FieldError errors={field.state.meta.errors} />
                        )}
                      </Field>
                    )
                  }}
                </form.Field>

                <form.Field name="name">
                  {(field) => {
                    const isInvalid = field.state.meta.isTouched && !field.state.meta.isValid
                    return (
                      <Field data-invalid={isInvalid}>
                        <FieldLabel htmlFor={field.name}>Name</FieldLabel>
                        <Input
                          aria-invalid={isInvalid}
                          id={field.name}
                          name={field.name}
                          value={field.state.value}
                          onBlur={field.handleBlur}
                          onChange={(e) => field.handleChange(e.target.value)}
                          placeholder="runner-1"
                        />
                        {field.state.meta.errors.length > 0 && field.state.meta.isTouched && (
                          <FieldError errors={field.state.meta.errors} />
                        )}
                      </Field>
                    )
                  }}
                </form.Field>
              </form>
            )}
          </div>
        </ScrollArea>

        <SheetFooter className="border-t border-border p-4 px-5">
          <Button type="button" variant="secondary" onClick={() => setOpen(false)}>
            {createdRunner ? 'Close' : 'Cancel'}
          </Button>
          {!createdRunner && (
            <form.Subscribe
              selector={(state) => [state.canSubmit, state.isSubmitting]}
              children={([canSubmit, isSubmitting]) => (
                <Button type="submit" form="create-runner-form" variant="default" disabled={!canSubmit || isSubmitting}>
                  {isSubmitting && <Spinner />}
                  Create
                </Button>
              )}
            />
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function CreatedRunnerDisplay({ createdRunner }: { createdRunner: CreateRunnerResponse }) {
  return (
    <div className="space-y-6">
      <Alert variant="warning">
        <InfoIcon />
        <AlertDescription>Runner created successfully.</AlertDescription>
      </Alert>
      <FieldGroup className="gap-4">
        <Field>
          <FieldLabel htmlFor="runner-id">Runner ID</FieldLabel>
          <Input id="runner-id" value={createdRunner.id} readOnly />
        </Field>
      </FieldGroup>
    </div>
  )
}
