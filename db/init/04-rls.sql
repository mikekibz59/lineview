-- -- The tenant for the current transaction. NULLIF matters; after a transaction that used
-- -- set_config(..., true), a pooled connection reports '' (empty string), not NULL,
-- -- and ''::uuid raises an error. NULLIF turns '' into NULL, so "no tenant set" -> zero rows.
-- 
CREATE OR REPLACE FUNCTION public.current_tenant() RETURNS uuid
LANGUAGE sql STABLE
AS $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;

GRANT EXECUTE ON FUNCTION public.current_tenant() TO lineview_app;

DO $$
DECLARE t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['machines', 'products', 'inspection_results'] LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON public.%I', t);
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON public.%I TO lineview_app
             USING (tenant_id = public.current_tenant())
             WITH CHECK (tenant_id = public.current_tenant())', t);
    END LOOP;
END $$;

-- tenant is keyed on id, not tenant_id
ALTER TABLE public.tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.tenants FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON public.tenants;
CREATE POLICY tenant_isolation ON public.tenants TO lineview_app
    USING (id = public.current_tenant())
    WITH CHECK (id = public.current_tenant());