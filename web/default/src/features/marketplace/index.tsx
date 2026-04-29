import { useCallback, useMemo, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import { FixedOrdersTab } from './components/fixed-orders-tab'
import { OrdersTab } from './components/orders-tab'
import { PoolTab } from './components/pool-tab'
import { SellerTab } from './components/seller-tab'

export function Marketplace() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState('orders')
  const tabItems = useMemo(
    () => [
      { value: 'orders', label: t('Order list') },
      { value: 'pool', label: t('Order pool') },
      { value: 'fixed', label: t('My fixed orders') },
      { value: 'seller', label: t('Seller desk') },
    ],
    [t]
  )
  const guideItems = useMemo(
    () => [
      {
        label: t('Sellers list'),
        text: t('Host keys, set models, prices, and concurrency'),
      },
      {
        label: t('Buyers purchase'),
        text: t('Buy fixed quota and bind tokens for fixed routing'),
      },
      {
        label: t('Order pool calls'),
        text: t('Automatically select supply by model, price, and concurrency'),
      },
    ],
    [t]
  )
  const openFixedOrders = useCallback(() => setActiveTab('fixed'), [])

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Marketplace')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        <span className='block max-w-3xl'>
          {t(
            'Marketplace turns available AI API keys into purchasable, routable AI capacity.'
          )}
        </span>
        <span className='mt-2 flex max-w-4xl flex-wrap gap-2'>
          {guideItems.map((item) => (
            <span
              key={item.label}
              className='bg-muted/40 inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs leading-5'
            >
              <span className='text-foreground font-medium'>{item.label}</span>
              <span>{item.text}</span>
            </span>
          ))}
        </span>
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <Button asChild variant='outline' size='sm'>
          <Link to='/keys'>
            <KeyRound className='size-4' />
            {t('Manage console tokens')}
          </Link>
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <Tabs value={activeTab} onValueChange={setActiveTab} className='gap-4'>
          <TabsList className='w-full justify-start overflow-x-auto sm:w-fit'>
            {tabItems.map((item) => (
              <TabsTrigger key={item.value} value={item.value}>
                {item.label}
              </TabsTrigger>
            ))}
          </TabsList>
          <TabsContent value='orders'>
            <OrdersTab onFixedOrderCreated={openFixedOrders} />
          </TabsContent>
          <TabsContent value='pool'>
            <PoolTab />
          </TabsContent>
          <TabsContent value='fixed'>
            <FixedOrdersTab />
          </TabsContent>
          <TabsContent value='seller'>
            <SellerTab />
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
