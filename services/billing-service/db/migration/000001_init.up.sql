CREATE TYPE subscription_status AS ENUM ('pending', 'active', 'cancelled', 'expired');
CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed', 'cancelled', 'refunded');

CREATE TABLE subscriptions (
    subscription_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    plan_code VARCHAR(50) NOT NULL,
    status subscription_status NOT NULL DEFAULT 'pending',
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_subscriptions_user_status
    ON subscriptions(user_id, status, current_period_end DESC);

CREATE TABLE payment_transactions (
    transaction_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    subscription_id UUID REFERENCES subscriptions(subscription_id),
    provider VARCHAR(20) NOT NULL DEFAULT 'payos',
    provider_payment_id TEXT,
    provider_order_code BIGINT NOT NULL,
    idempotency_key TEXT NOT NULL,
    amount_vnd BIGINT NOT NULL,
    status payment_status NOT NULL DEFAULT 'pending',
    checkout_url TEXT,
    raw_payload JSONB,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, provider_order_code),
    UNIQUE(idempotency_key)
);

CREATE UNIQUE INDEX idx_payment_transactions_provider_payment_id
    ON payment_transactions(provider, provider_payment_id)
    WHERE provider_payment_id IS NOT NULL;

CREATE INDEX idx_payment_transactions_user_created
    ON payment_transactions(user_id, created_at DESC);

CREATE TABLE payment_webhook_events (
    webhook_event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(20) NOT NULL DEFAULT 'payos',
    event_key TEXT NOT NULL,
    raw_payload JSONB NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, event_key)
);
