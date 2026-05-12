import { describe, expect, it } from 'bun:test'
import { getUserSubscriptionDisplayStatus } from '../../src/features/subscriptions/lib/status'
import type { UserSubscription } from '../../src/features/subscriptions/types'

function subscription(patch: Partial<UserSubscription>): UserSubscription {
  return {
    id: 1,
    user_id: 94,
    plan_id: 1,
    status: 'active',
    source: 'order',
    start_time: 1_800_000_000,
    end_time: 1_900_000_000,
    amount_total: 5000000,
    amount_used: 1000000,
    ...patch,
  }
}

describe('getUserSubscriptionDisplayStatus', () => {
  const now = 1_850_000_000

  it('shows quota exhaustion separately from date expiry', () => {
    expect(
      getUserSubscriptionDisplayStatus(
        subscription({
          status: 'expired',
          end_time: now + 3600,
          amount_total: 5000000,
          amount_used: 5000000,
        }),
        now
      )
    ).toBe('exhausted')
  })

  it('keeps date-expired subscriptions expired even when quota is also full', () => {
    expect(
      getUserSubscriptionDisplayStatus(
        subscription({
          status: 'expired',
          end_time: now - 1,
          amount_total: 5000000,
          amount_used: 5000000,
        }),
        now
      )
    ).toBe('expired')
  })

  it('keeps active future subscriptions active while quota remains', () => {
    expect(
      getUserSubscriptionDisplayStatus(
        subscription({
          status: 'active',
          end_time: now + 3600,
          amount_total: 5000000,
          amount_used: 4999999,
        }),
        now
      )
    ).toBe('active')
  })

  it('treats unlimited quota subscriptions as active before expiry', () => {
    expect(
      getUserSubscriptionDisplayStatus(
        subscription({
          status: 'active',
          end_time: now + 3600,
          amount_total: 0,
          amount_used: 5000000,
        }),
        now
      )
    ).toBe('active')
  })

  it('keeps cancelled subscriptions invalidated', () => {
    expect(
      getUserSubscriptionDisplayStatus(
        subscription({
          status: 'cancelled',
          end_time: now + 3600,
          amount_total: 5000000,
          amount_used: 5000000,
        }),
        now
      )
    ).toBe('cancelled')
  })
})
