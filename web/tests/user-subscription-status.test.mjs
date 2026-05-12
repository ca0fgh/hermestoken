import test from 'node:test';
import assert from 'node:assert/strict';

const now = 1_850_000_000;

function subscription(patch = {}) {
  return {
    id: 54,
    status: 'active',
    end_time: now + 3600,
    amount_total: 5_000_000,
    amount_used: 1_000_000,
    ...patch,
  };
}

test('classic user subscription display separates exhausted from expired', async () => {
  const { getUserSubscriptionDisplayStatus } = await import(
    '../classic/src/helpers/subscriptionStatus.js'
  );

  assert.equal(
    getUserSubscriptionDisplayStatus(
      subscription({
        status: 'expired',
        amount_used: 5_000_000,
      }),
      now,
    ),
    'exhausted',
  );
});

test('classic user subscription display keeps true time expiry expired', async () => {
  const { getUserSubscriptionDisplayStatus } = await import(
    '../classic/src/helpers/subscriptionStatus.js'
  );

  assert.equal(
    getUserSubscriptionDisplayStatus(
      subscription({
        status: 'expired',
        end_time: now - 1,
        amount_used: 5_000_000,
      }),
      now,
    ),
    'expired',
  );
});

test('classic user subscription display keeps active subscriptions active', async () => {
  const { getUserSubscriptionDisplayStatus } = await import(
    '../classic/src/helpers/subscriptionStatus.js'
  );

  assert.equal(getUserSubscriptionDisplayStatus(subscription(), now), 'active');
});

test('classic user subscription display keeps cancelled subscriptions invalidated', async () => {
  const { getUserSubscriptionDisplayStatus } = await import(
    '../classic/src/helpers/subscriptionStatus.js'
  );

  assert.equal(
    getUserSubscriptionDisplayStatus(
      subscription({
        status: 'cancelled',
        amount_used: 5_000_000,
      }),
      now,
    ),
    'cancelled',
  );
});
