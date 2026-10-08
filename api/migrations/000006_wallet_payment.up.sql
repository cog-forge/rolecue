BEGIN;

CREATE TYPE public.transaction_status AS ENUM ('pending', 'success', 'failed', 'cancelled');

CREATE TABLE public.wallets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL UNIQUE REFERENCES public.users(id),
    balance integer NOT NULL DEFAULT 0
);

CREATE TABLE public.transactions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    amount integer NOT NULL,
    "to" text NOT NULL,
    description text,
    currency text NOT NULL,
    status public.transaction_status NOT NULL DEFAULT 'pending',
    payos_order_code text,
    "from" text NOT NULL
);

CREATE TABLE public.coin_packages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    price integer NOT NULL,
    description text,
    amount integer NOT NULL,
    currency text NOT NULL
);

COMMIT;
