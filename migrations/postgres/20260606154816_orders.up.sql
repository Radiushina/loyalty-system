CREATE TYPE order_status AS ENUM('NEW', 'PROCESSING', 'INVALID', 'PROCESSED');

CREATE TABLE IF NOT EXISTS orders(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    number TEXT NOT NULL UNIQUE,
    status order_status NOT NULL DEFAULT 'NEW'::order_status,
    accrual NUMERIC NULL,
    uploaded_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT orders_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE INDEX IF NOT EXISTS orders_user_id_uploaded_at_idx
    ON orders (user_id, uploaded_at DESC);