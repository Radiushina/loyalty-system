CREATE TABLE IF NOT EXISTS balance_accruals(
    order_id    UUID PRIMARY KEY,
    user_id     UUID NOT NULL,
    amount      NUMERIC NOT NULL,
    credited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT balance_accruals_order_id_fkey
        FOREIGN KEY (order_id) REFERENCES orders (id),
    CONSTRAINT balance_accruals_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id)
);
