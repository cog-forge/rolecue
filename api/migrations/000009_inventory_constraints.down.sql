BEGIN;

ALTER TABLE public.inventories
    DROP CONSTRAINT inventories_slot_nonnegative,
    DROP CONSTRAINT inventories_user_id_unique;

COMMIT;
