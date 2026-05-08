import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const vendorTypesSchema = z.string().refine((value) => {
  try {
    const parsed = JSON.parse(value)
    return (
      Array.isArray(parsed) &&
      parsed.every((item) => Number.isInteger(item) && item > 0)
    )
  } catch {
    return false
  }
}, 'Must be a JSON array of channel type ids')

const marketplaceSchema = z.object({
  MarketplaceEnabled: z.boolean(),
  MarketplaceEnabledVendorTypes: vendorTypesSchema,
  MarketplaceFeeRate: z.coerce.number().min(0),
  MarketplaceSellerIncomeHoldSeconds: z.coerce.number().int().min(0),
  MarketplaceMinFixedOrderQuota: z.coerce.number().int().min(0),
  MarketplaceMaxFixedOrderQuota: z.coerce.number().int().min(0),
  MarketplaceFixedOrderDefaultExpirySeconds: z.coerce.number().int().min(1),
  MarketplaceMaxSellerMultiplier: z.coerce.number().min(0.01),
  MarketplaceMaxCredentialConcurrency: z.coerce.number().int().min(0),
})

type MarketplaceSettingsFormValues = z.infer<typeof marketplaceSchema>

type MarketplaceSettingsSectionProps = {
  defaultValues: MarketplaceSettingsFormValues
}

type MarketplaceNumberField = {
  name: Exclude<
    keyof MarketplaceSettingsFormValues,
    'MarketplaceEnabled' | 'MarketplaceEnabledVendorTypes'
  >
  label: string
  description: string
  min: number
  max?: number
  step: number
}

const numberFields: MarketplaceNumberField[] = [
  {
    name: 'MarketplaceFeeRate',
    label: 'Buyer transaction fee rate',
    description: 'Charged on every marketplace buyer call; 0.05 means 5%',
    min: 0,
    step: 0.0001,
  },
  {
    name: 'MarketplaceSellerIncomeHoldSeconds',
    label: 'Income hold seconds',
    description: 'Seller income release delay',
    min: 0,
    step: 1,
  },
  {
    name: 'MarketplaceMinFixedOrderQuota',
    label: 'Minimum buyout quota',
    description: '0 disables the lower bound',
    min: 0,
    step: 1,
  },
  {
    name: 'MarketplaceMaxFixedOrderQuota',
    label: 'Maximum buyout quota',
    description: '0 disables the upper bound',
    min: 0,
    step: 1,
  },
  {
    name: 'MarketplaceFixedOrderDefaultExpirySeconds',
    label: 'Buyout expiry seconds',
    description: 'Fixed orders expire at this age',
    min: 1,
    step: 1,
  },
  {
    name: 'MarketplaceMaxSellerMultiplier',
    label: 'Maximum seller multiplier',
    description: 'Upper bound for seller pricing',
    min: 0.01,
    step: 0.01,
  },
  {
    name: 'MarketplaceMaxCredentialConcurrency',
    label: 'Maximum credential concurrency',
    description:
      'Upper bound for seller routing concurrency; 0 disables the upper bound',
    min: 0,
    step: 1,
  },
]

export function MarketplaceSettingsSection({
  defaultValues,
}: MarketplaceSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<MarketplaceSettingsFormValues>({
      resolver: zodResolver(marketplaceSchema) as Resolver<
        MarketplaceSettingsFormValues,
        unknown,
        MarketplaceSettingsFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key,
            value: value as string | number | boolean,
          })
        }
      },
    })

  return (
    <SettingsSection
      title={t('Marketplace Settings')}
      description={t('Configure marketplace availability and trade limits')}
    >
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <form onSubmit={handleSubmit} className='space-y-6'>
          <FormDirtyIndicator isDirty={isDirty} />
          <FormField
            control={form.control}
            name='MarketplaceEnabled'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between gap-4 rounded-md border p-4'>
                <div>
                  <FormLabel>{t('Enable marketplace')}</FormLabel>
                  <FormDescription>
                    {t('Allow sellers and buyers to use marketplace flows')}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='MarketplaceEnabledVendorTypes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Enabled vendor types')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    value={field.value}
                    onChange={field.onChange}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t('Existing channel type ids accepted by seller escrow')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 md:grid-cols-2'>
            {numberFields.map((item) => (
              <FormField
                key={item.name}
                control={form.control}
                name={item.name}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t(item.label)}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={item.min}
                        max={'max' in item ? item.max : undefined}
                        step={item.step}
                        value={field.value as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>{t(item.description)}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ))}
          </div>

          <div className='flex justify-end'>
            <Button type='submit' disabled={!isDirty || isSubmitting}>
              {isSubmitting ? t('Saving...') : t('Save Settings')}
            </Button>
          </div>
        </form>
      </Form>
    </SettingsSection>
  )
}
