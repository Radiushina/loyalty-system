CREATE TABLE IF NOT EXISTS withdrawals(
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    order_number TEXT NOT NULL,
    sum          NUMERIC NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT withdrawals_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT withdrawals_user_order_unique
        UNIQUE (user_id, order_number)
);

CREATE INDEX withdrawals_user_id_processed_at_idx
    ON withdrawals (user_id, processed_at DESC);