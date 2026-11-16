-- Reverts 000036. avatars.model_path comes back from the asset each avatar points at, so nothing is lost.
-- Listings are dropped with their data; assets go last.
BEGIN;

ALTER TABLE public.avatars ADD COLUMN model_path text;
UPDATE public.avatars a SET model_path = s.model_path FROM public.avatar_assets s WHERE s.id = a.asset_id;
ALTER TABLE public.avatars ALTER COLUMN model_path SET NOT NULL;

DROP INDEX public.avatars_asset_id_idx;
ALTER TABLE public.avatars DROP CONSTRAINT avatars_asset_id_fkey, DROP COLUMN asset_id;

DROP TABLE public.avatar_listings;
DROP TABLE public.avatar_assets;

COMMIT;
