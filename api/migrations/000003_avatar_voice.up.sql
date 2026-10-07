BEGIN;

CREATE TABLE public.inventories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES public.users(id),
    slot integer NOT NULL
);
CREATE INDEX inventories_user_id_idx ON public.inventories(user_id);

CREATE TABLE public.avatars (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id uuid NOT NULL REFERENCES public.inventories(id),
    name text NOT NULL,
    model_path text NOT NULL,
    personality text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX avatars_inventory_id_idx ON public.avatars(inventory_id);

CREATE TABLE public.voices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    is_active boolean NOT NULL DEFAULT true
);

COMMIT;
