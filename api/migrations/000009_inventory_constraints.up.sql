BEGIN;

ALTER TABLE public.inventories
    ADD CONSTRAINT inventories_user_id_unique UNIQUE (user_id),
    ADD CONSTRAINT inventories_slot_nonnegative CHECK (slot >= 0);

COMMIT;
