-- =============================================================================
-- RoleCue Billing Seed Data: Users, Wallets & 90-Day Transactions
-- Generated: 2026-10-21T10:11:53.667Z
-- =============================================================================
BEGIN;

-- 0. Clean prior seed transactions (identified by order code prefix '26071' or seed descriptions)

DELETE FROM public.transactions
WHERE (payos_order_code IS NOT NULL AND payos_order_code LIKE '26071%')
   OR description LIKE '%(Frontend React)%'
   OR description LIKE '%(Golang Backend)%'
   OR description LIKE '%(Distributed Systems)%'
   OR description LIKE '%(Leadership Principles)%'
   OR description LIKE '%package reversal%'
   OR description LIKE '%credit compensation%';

-- 1. Insert Seed Users

INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000001',
  'Nguyễn Hoàng Nam',
  'nam.nguyen@gmail.com',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '88 days',
  NOW() - INTERVAL '88 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000002',
  'Trần Thị Minh Anh',
  'minhanh.tran@outlook.com',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '84 days',
  NOW() - INTERVAL '84 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000003',
  'Lê Quốc Bảo',
  'bao.le@techdev.io',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '79 days',
  NOW() - INTERVAL '79 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000004',
  'Phan Bích Ngọc',
  'ngoc.phan@fpt.com',
  true,
  'recruiter',
  'completed',
  'FPT Software',
  'https://fpt-software.com',
  NOW() - INTERVAL '75 days',
  NOW() - INTERVAL '75 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000005',
  'Phạm Thu Hương',
  'huong.pham@fpt.edu.vn',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '69 days',
  NOW() - INTERVAL '69 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000006',
  'Trương Hữu Nghĩa',
  'nghia.truong@viettel.vn',
  true,
  'recruiter',
  'completed',
  'Viettel Solutions',
  'https://solutions.viettel.vn',
  NOW() - INTERVAL '64 days',
  NOW() - INTERVAL '64 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000007',
  'Võ Đức Trí',
  'tri.vo@gmail.com',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '58 days',
  NOW() - INTERVAL '58 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000008',
  'Alex Johnson',
  'alex.j@vng.com.vn',
  true,
  'recruiter',
  'completed',
  'VNG Corporation',
  'https://vng.com.vn',
  NOW() - INTERVAL '53 days',
  NOW() - INTERVAL '53 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000009',
  'Đỗ Mỹ Linh',
  'linh.do@hust.edu.vn',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '47 days',
  NOW() - INTERVAL '47 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000010',
  'Bùi Văn Thành',
  'thanh.bui@vnu.edu.vn',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '41 days',
  NOW() - INTERVAL '41 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000011',
  'Elena Rostova',
  'elena@tiki.vn',
  true,
  'recruiter',
  'completed',
  'Tiki Corp',
  'https://tiki.vn',
  NOW() - INTERVAL '35 days',
  NOW() - INTERVAL '35 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000012',
  'Hoàng Yến Nhi',
  'nhi.hoang@gmail.com',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '29 days',
  NOW() - INTERVAL '29 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000013',
  'Vũ Minh Quân',
  'quan.vu@onemount.com',
  true,
  'recruiter',
  'completed',
  'One Mount Group',
  'https://onemount.com',
  NOW() - INTERVAL '21 days',
  NOW() - INTERVAL '21 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000014',
  'Ngô Gia Huy',
  'huy.ngo@uit.edu.vn',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '15 days',
  NOW() - INTERVAL '15 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000015',
  'Đặng Phương Thảo',
  'thao.dang@gmail.com',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '9 days',
  NOW() - INTERVAL '9 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '11111111-1111-4000-8000-000000000016',
  'Nguyễn Hải Đăng',
  'dang.nguyen@gmail.com',
  true,
  'candidate',
  'completed',
  NULL,
  NULL,
  NOW() - INTERVAL '3 days',
  NOW() - INTERVAL '3 days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;


-- 2. Insert Wallets for Seed Users

INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000001',
  75
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000002',
  150
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000003',
  225
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000004',
  300
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000005',
  75
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000006',
  150
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000007',
  225
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000008',
  300
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000009',
  75
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000010',
  150
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000011',
  225
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000012',
  300
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000013',
  75
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000014',
  150
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000015',
  225
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '11111111-1111-4000-8000-000000000016',
  300
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;


-- 3. Seed Transactions Across Last 90 Days

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000001',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000001',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710001',
  'plink_260710001',
  NULL,
  100,
  200,
  TRUE,
  NOW() - INTERVAL '87 days + 0 hours + 0 minutes + 0 seconds',
  NOW() - INTERVAL '87 days + 0 hours + 0 minutes + 0 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000002',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000002',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710002',
  'plink_260710002',
  NULL,
  125,
  375,
  FALSE,
  NOW() - INTERVAL '84 days + 7 hours + 17 minutes + 29 seconds',
  NOW() - INTERVAL '84 days + 7 hours + 17 minutes + 29 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000003',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000003',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710003',
  'plink_260710003',
  NULL,
  150,
  150,
  FALSE,
  NOW() - INTERVAL '81 days + 14 hours + 34 minutes + 58 seconds',
  NOW() - INTERVAL '81 days + 14 hours + 34 minutes + 58 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000004',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000004',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710004',
  'plink_260710004',
  NULL,
  175,
  275,
  FALSE,
  NOW() - INTERVAL '78 days + 21 hours + 51 minutes + 27 seconds',
  NOW() - INTERVAL '78 days + 21 hours + 51 minutes + 27 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000005',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000005',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710005',
  'plink_260710005',
  NULL,
  200,
  450,
  FALSE,
  NOW() - INTERVAL '75 days + 4 hours + 8 minutes + 56 seconds',
  NOW() - INTERVAL '75 days + 4 hours + 8 minutes + 56 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000006',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000006',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'failed',
  '260710006',
  'plink_260710006',
  'Payment authorization declined by customer issuing bank (Card response: RC_51)',
  225,
  225,
  FALSE,
  NOW() - INTERVAL '72 days + 11 hours + 25 minutes + 25 seconds',
  NOW() - INTERVAL '72 days + 11 hours + 25 minutes + 25 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000007',
  100,
  '11111111-1111-4000-8000-000000000007',
  'System',
  'Recruiter Candidate Assessment Screening Batch',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  340,
  240,
  FALSE,
  NOW() - INTERVAL '69 days + 18 hours + 42 minutes + 54 seconds',
  NOW() - INTERVAL '69 days + 18 hours + 42 minutes + 54 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000008',
  75,
  '11111111-1111-4000-8000-000000000008',
  'System',
  'Technical Evaluation Report Generation',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  355,
  280,
  FALSE,
  NOW() - INTERVAL '66 days + 1 hours + 59 minutes + 23 seconds',
  NOW() - INTERVAL '66 days + 1 hours + 59 minutes + 23 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000009',
  40,
  '11111111-1111-4000-8000-000000000009',
  'System',
  'Live Coding Sandbox Practice Session',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  370,
  330,
  FALSE,
  NOW() - INTERVAL '63 days + 8 hours + 16 minutes + 52 seconds',
  NOW() - INTERVAL '63 days + 8 hours + 16 minutes + 52 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000010',
  50000,
  'System',
  '11111111-1111-4000-8000-000000000010',
  'Refund: Cancelled mock interview session (50000 VND)',
  'VND',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  75,
  '22222222-2222-4000-8000-000000000004',
  TRUE,
  FALSE,
  NOW() - INTERVAL '60 days + 15 hours + 33 minutes + 21 seconds',
  NOW() - INTERVAL '60 days + 15 hours + 33 minutes + 21 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000011',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000011',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710007',
  'plink_260710007',
  NULL,
  100,
  350,
  FALSE,
  NOW() - INTERVAL '59 days + 22 hours + 50 minutes + 50 seconds',
  NOW() - INTERVAL '59 days + 22 hours + 50 minutes + 50 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000012',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000012',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'cancelled',
  '260710008',
  'plink_260710008',
  'Customer cancelled transaction on PayOS payment checkout',
  125,
  125,
  FALSE,
  NOW() - INTERVAL '58 days + 5 hours + 7 minutes + 19 seconds',
  NOW() - INTERVAL '58 days + 5 hours + 7 minutes + 19 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000013',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000013',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710009',
  'plink_260710009',
  NULL,
  150,
  250,
  FALSE,
  NOW() - INTERVAL '57 days + 12 hours + 24 minutes + 48 seconds',
  NOW() - INTERVAL '57 days + 12 hours + 24 minutes + 48 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000014',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000014',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710010',
  'plink_260710010',
  NULL,
  175,
  425,
  FALSE,
  NOW() - INTERVAL '56 days + 19 hours + 41 minutes + 17 seconds',
  NOW() - INTERVAL '56 days + 19 hours + 41 minutes + 17 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000015',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000015',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710011',
  'plink_260710011',
  NULL,
  200,
  200,
  FALSE,
  NOW() - INTERVAL '55 days + 2 hours + 58 minutes + 46 seconds',
  NOW() - INTERVAL '55 days + 2 hours + 58 minutes + 46 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000016',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000016',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710012',
  'plink_260710012',
  NULL,
  225,
  325,
  FALSE,
  NOW() - INTERVAL '54 days + 9 hours + 15 minutes + 15 seconds',
  NOW() - INTERVAL '54 days + 9 hours + 15 minutes + 15 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000017',
  75,
  'a7daf95f-5c68-4ef4-ad62-5166079dbff1',
  'System',
  'Technical Evaluation Report Generation',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  490,
  415,
  FALSE,
  NOW() - INTERVAL '53 days + 16 hours + 32 minutes + 44 seconds',
  NOW() - INTERVAL '53 days + 16 hours + 32 minutes + 44 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000018',
  40,
  'c21173fc-ea76-496b-a688-f3aa03f747d5',
  'System',
  'Live Coding Sandbox Practice Session',
  'COIN',
  'failed',
  NULL,
  NULL,
  'Insufficient coin balance during mock interview launch',
  505,
  505,
  FALSE,
  NOW() - INTERVAL '52 days + 23 hours + 49 minutes + 13 seconds',
  NOW() - INTERVAL '52 days + 23 hours + 49 minutes + 13 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000019',
  20,
  '1820709c-d248-4b60-ab6c-ff0cce37508b',
  'System',
  'AI Technical Mock Interview Session (Frontend React)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  520,
  500,
  FALSE,
  NOW() - INTERVAL '51 days + 6 hours + 6 minutes + 42 seconds',
  NOW() - INTERVAL '51 days + 6 hours + 6 minutes + 42 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000020',
  100000,
  'System',
  '11111111-1111-4000-8000-000000000001',
  'Refund: Pro practice package reversal (100000 VND)',
  'VND',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  75,
  '22222222-2222-4000-8000-000000000012',
  TRUE,
  FALSE,
  NOW() - INTERVAL '50 days + 13 hours + 23 minutes + 11 seconds',
  NOW() - INTERVAL '50 days + 13 hours + 23 minutes + 11 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000021',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000002',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710013',
  'plink_260710013',
  NULL,
  100,
  100,
  FALSE,
  NOW() - INTERVAL '49 days + 20 hours + 40 minutes + 40 seconds',
  NOW() - INTERVAL '49 days + 20 hours + 40 minutes + 40 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000022',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000003',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710014',
  'plink_260710014',
  NULL,
  125,
  225,
  FALSE,
  NOW() - INTERVAL '48 days + 3 hours + 57 minutes + 9 seconds',
  NOW() - INTERVAL '48 days + 3 hours + 57 minutes + 9 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000023',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000004',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710015',
  'plink_260710015',
  NULL,
  150,
  400,
  FALSE,
  NOW() - INTERVAL '47 days + 10 hours + 14 minutes + 38 seconds',
  NOW() - INTERVAL '47 days + 10 hours + 14 minutes + 38 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000024',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000005',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'cancelled',
  '260710016',
  'plink_260710016',
  'Customer cancelled transaction on PayOS payment checkout',
  175,
  175,
  FALSE,
  NOW() - INTERVAL '46 days + 17 hours + 31 minutes + 7 seconds',
  NOW() - INTERVAL '46 days + 17 hours + 31 minutes + 7 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000025',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000006',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710017',
  'plink_260710017',
  NULL,
  200,
  300,
  FALSE,
  NOW() - INTERVAL '45 days + 0 hours + 48 minutes + 36 seconds',
  NOW() - INTERVAL '45 days + 0 hours + 48 minutes + 36 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000026',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000007',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710018',
  'plink_260710018',
  NULL,
  225,
  475,
  FALSE,
  NOW() - INTERVAL '44 days + 7 hours + 5 minutes + 5 seconds',
  NOW() - INTERVAL '44 days + 7 hours + 5 minutes + 5 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000027',
  40,
  '11111111-1111-4000-8000-000000000008',
  'System',
  'Live Coding Sandbox Practice Session',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  340,
  300,
  FALSE,
  NOW() - INTERVAL '43 days + 14 hours + 22 minutes + 34 seconds',
  NOW() - INTERVAL '43 days + 14 hours + 22 minutes + 34 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000028',
  20,
  '11111111-1111-4000-8000-000000000009',
  'System',
  'AI Technical Mock Interview Session (Frontend React)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  355,
  335,
  FALSE,
  NOW() - INTERVAL '42 days + 21 hours + 39 minutes + 3 seconds',
  NOW() - INTERVAL '42 days + 21 hours + 39 minutes + 3 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000029',
  25,
  '11111111-1111-4000-8000-000000000010',
  'System',
  'AI Technical Mock Interview Session (Golang Backend)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  370,
  345,
  FALSE,
  NOW() - INTERVAL '41 days + 4 hours + 56 minutes + 32 seconds',
  NOW() - INTERVAL '41 days + 4 hours + 56 minutes + 32 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000030',
  25,
  'System',
  '11111111-1111-4000-8000-000000000011',
  'Refund: System disruption credit compensation',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  100,
  '22222222-2222-4000-8000-000000000016',
  FALSE,
  FALSE,
  NOW() - INTERVAL '40 days + 11 hours + 13 minutes + 1 seconds',
  NOW() - INTERVAL '40 days + 11 hours + 13 minutes + 1 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000031',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000012',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710019',
  'plink_260710019',
  NULL,
  100,
  200,
  FALSE,
  NOW() - INTERVAL '39 days + 18 hours + 30 minutes + 30 seconds',
  NOW() - INTERVAL '39 days + 18 hours + 30 minutes + 30 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000032',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000013',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710020',
  'plink_260710020',
  NULL,
  125,
  375,
  FALSE,
  NOW() - INTERVAL '38 days + 1 hours + 47 minutes + 59 seconds',
  NOW() - INTERVAL '38 days + 1 hours + 47 minutes + 59 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000033',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000014',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710021',
  'plink_260710021',
  NULL,
  150,
  150,
  FALSE,
  NOW() - INTERVAL '37 days + 8 hours + 4 minutes + 28 seconds',
  NOW() - INTERVAL '37 days + 8 hours + 4 minutes + 28 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000034',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000015',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710022',
  'plink_260710022',
  NULL,
  175,
  275,
  FALSE,
  NOW() - INTERVAL '36 days + 15 hours + 21 minutes + 57 seconds',
  NOW() - INTERVAL '36 days + 15 hours + 21 minutes + 57 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000035',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000016',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710023',
  'plink_260710023',
  NULL,
  200,
  450,
  FALSE,
  NOW() - INTERVAL '35 days + 22 hours + 38 minutes + 26 seconds',
  NOW() - INTERVAL '35 days + 22 hours + 38 minutes + 26 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000036',
  600,
  'Banking (PayOS)',
  'a7daf95f-5c68-4ef4-ad62-5166079dbff1',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'cancelled',
  '260710024',
  'plink_260710024',
  'Customer cancelled transaction on PayOS payment checkout',
  225,
  225,
  FALSE,
  NOW() - INTERVAL '34 days + 5 hours + 55 minutes + 55 seconds',
  NOW() - INTERVAL '34 days + 5 hours + 55 minutes + 55 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000037',
  20,
  'c21173fc-ea76-496b-a688-f3aa03f747d5',
  'System',
  'AI Technical Mock Interview Session (Frontend React)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  490,
  470,
  FALSE,
  NOW() - INTERVAL '33 days + 12 hours + 12 minutes + 24 seconds',
  NOW() - INTERVAL '33 days + 12 hours + 12 minutes + 24 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000038',
  25,
  '1820709c-d248-4b60-ab6c-ff0cce37508b',
  'System',
  'AI Technical Mock Interview Session (Golang Backend)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  505,
  480,
  FALSE,
  NOW() - INTERVAL '32 days + 19 hours + 29 minutes + 53 seconds',
  NOW() - INTERVAL '32 days + 19 hours + 29 minutes + 53 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000039',
  30,
  '11111111-1111-4000-8000-000000000001',
  'System',
  'System Design Mock Interview (Distributed Systems)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  520,
  490,
  FALSE,
  NOW() - INTERVAL '31 days + 2 hours + 46 minutes + 22 seconds',
  NOW() - INTERVAL '31 days + 2 hours + 46 minutes + 22 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000040',
  50000,
  'System',
  '11111111-1111-4000-8000-000000000002',
  'Refund: Cancelled mock interview session (50000 VND)',
  'VND',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  75,
  '22222222-2222-4000-8000-000000000024',
  TRUE,
  FALSE,
  NOW() - INTERVAL '30 days + 9 hours + 3 minutes + 51 seconds',
  NOW() - INTERVAL '30 days + 9 hours + 3 minutes + 51 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000041',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000003',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710025',
  'plink_260710025',
  NULL,
  100,
  350,
  FALSE,
  NOW() - INTERVAL '29 days + 16 hours + 20 minutes + 20 seconds',
  NOW() - INTERVAL '29 days + 16 hours + 20 minutes + 20 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000042',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000004',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'failed',
  '260710026',
  'plink_260710026',
  'Payment authorization declined by customer issuing bank (Card response: RC_51)',
  125,
  125,
  FALSE,
  NOW() - INTERVAL '29 days + 23 hours + 37 minutes + 49 seconds',
  NOW() - INTERVAL '29 days + 23 hours + 37 minutes + 49 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000043',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000005',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710027',
  'plink_260710027',
  NULL,
  150,
  250,
  FALSE,
  NOW() - INTERVAL '28 days + 6 hours + 54 minutes + 18 seconds',
  NOW() - INTERVAL '28 days + 6 hours + 54 minutes + 18 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000044',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000006',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710028',
  'plink_260710028',
  NULL,
  175,
  425,
  FALSE,
  NOW() - INTERVAL '28 days + 13 hours + 11 minutes + 47 seconds',
  NOW() - INTERVAL '28 days + 13 hours + 11 minutes + 47 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000045',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000007',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710029',
  'plink_260710029',
  NULL,
  200,
  200,
  FALSE,
  NOW() - INTERVAL '27 days + 20 hours + 28 minutes + 16 seconds',
  NOW() - INTERVAL '27 days + 20 hours + 28 minutes + 16 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000046',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000008',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710030',
  'plink_260710030',
  NULL,
  225,
  325,
  FALSE,
  NOW() - INTERVAL '27 days + 3 hours + 45 minutes + 45 seconds',
  NOW() - INTERVAL '27 days + 3 hours + 45 minutes + 45 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000047',
  25,
  '11111111-1111-4000-8000-000000000009',
  'System',
  'AI Technical Mock Interview Session (Golang Backend)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  340,
  315,
  FALSE,
  NOW() - INTERVAL '26 days + 10 hours + 2 minutes + 14 seconds',
  NOW() - INTERVAL '26 days + 10 hours + 2 minutes + 14 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000048',
  30,
  '11111111-1111-4000-8000-000000000010',
  'System',
  'System Design Mock Interview (Distributed Systems)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  355,
  325,
  FALSE,
  NOW() - INTERVAL '26 days + 17 hours + 19 minutes + 43 seconds',
  NOW() - INTERVAL '26 days + 17 hours + 19 minutes + 43 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000049',
  25,
  '11111111-1111-4000-8000-000000000011',
  'System',
  'Behavioral Interview Evaluation (Leadership Principles)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  370,
  345,
  FALSE,
  NOW() - INTERVAL '25 days + 0 hours + 36 minutes + 12 seconds',
  NOW() - INTERVAL '25 days + 0 hours + 36 minutes + 12 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000050',
  100000,
  'System',
  '11111111-1111-4000-8000-000000000012',
  'Refund: Pro practice package reversal (100000 VND)',
  'VND',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  75,
  '22222222-2222-4000-8000-000000000032',
  TRUE,
  FALSE,
  NOW() - INTERVAL '25 days + 7 hours + 53 minutes + 41 seconds',
  NOW() - INTERVAL '25 days + 7 hours + 53 minutes + 41 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000051',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000013',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710031',
  'plink_260710031',
  NULL,
  100,
  100,
  FALSE,
  NOW() - INTERVAL '24 days + 14 hours + 10 minutes + 10 seconds',
  NOW() - INTERVAL '24 days + 14 hours + 10 minutes + 10 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000052',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000014',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710032',
  'plink_260710032',
  NULL,
  125,
  225,
  FALSE,
  NOW() - INTERVAL '24 days + 21 hours + 27 minutes + 39 seconds',
  NOW() - INTERVAL '24 days + 21 hours + 27 minutes + 39 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000053',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000015',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710033',
  'plink_260710033',
  NULL,
  150,
  400,
  FALSE,
  NOW() - INTERVAL '23 days + 4 hours + 44 minutes + 8 seconds',
  NOW() - INTERVAL '23 days + 4 hours + 44 minutes + 8 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000054',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000016',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'failed',
  '260710034',
  'plink_260710034',
  'Payment authorization declined by customer issuing bank (Card response: RC_51)',
  175,
  175,
  FALSE,
  NOW() - INTERVAL '23 days + 11 hours + 1 minutes + 37 seconds',
  NOW() - INTERVAL '23 days + 11 hours + 1 minutes + 37 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000055',
  100,
  'Banking (PayOS)',
  'a7daf95f-5c68-4ef4-ad62-5166079dbff1',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710035',
  'plink_260710035',
  NULL,
  200,
  300,
  FALSE,
  NOW() - INTERVAL '22 days + 18 hours + 18 minutes + 6 seconds',
  NOW() - INTERVAL '22 days + 18 hours + 18 minutes + 6 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000056',
  250,
  'Banking (PayOS)',
  'c21173fc-ea76-496b-a688-f3aa03f747d5',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710036',
  'plink_260710036',
  NULL,
  225,
  475,
  FALSE,
  NOW() - INTERVAL '22 days + 1 hours + 35 minutes + 35 seconds',
  NOW() - INTERVAL '22 days + 1 hours + 35 minutes + 35 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000057',
  30,
  '1820709c-d248-4b60-ab6c-ff0cce37508b',
  'System',
  'System Design Mock Interview (Distributed Systems)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  490,
  460,
  FALSE,
  NOW() - INTERVAL '21 days + 8 hours + 52 minutes + 4 seconds',
  NOW() - INTERVAL '21 days + 8 hours + 52 minutes + 4 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000058',
  25,
  '11111111-1111-4000-8000-000000000001',
  'System',
  'Behavioral Interview Evaluation (Leadership Principles)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  505,
  480,
  FALSE,
  NOW() - INTERVAL '21 days + 15 hours + 9 minutes + 33 seconds',
  NOW() - INTERVAL '21 days + 15 hours + 9 minutes + 33 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000059',
  15,
  '11111111-1111-4000-8000-000000000002',
  'System',
  'Resume Deep Review & AI Feedback',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  520,
  505,
  FALSE,
  NOW() - INTERVAL '20 days + 22 hours + 26 minutes + 2 seconds',
  NOW() - INTERVAL '20 days + 22 hours + 26 minutes + 2 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000060',
  25,
  'System',
  '11111111-1111-4000-8000-000000000003',
  'Refund: System disruption credit compensation',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  100,
  '22222222-2222-4000-8000-000000000036',
  FALSE,
  FALSE,
  NOW() - INTERVAL '20 days + 5 hours + 43 minutes + 31 seconds',
  NOW() - INTERVAL '20 days + 5 hours + 43 minutes + 31 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000061',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000004',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710037',
  'plink_260710037',
  NULL,
  100,
  200,
  FALSE,
  NOW() - INTERVAL '19 days + 12 hours + 0 minutes + 0 seconds',
  NOW() - INTERVAL '19 days + 12 hours + 0 minutes + 0 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000062',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000005',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710038',
  'plink_260710038',
  NULL,
  125,
  375,
  FALSE,
  NOW() - INTERVAL '19 days + 19 hours + 17 minutes + 29 seconds',
  NOW() - INTERVAL '19 days + 19 hours + 17 minutes + 29 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000063',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000006',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710039',
  'plink_260710039',
  NULL,
  150,
  150,
  FALSE,
  NOW() - INTERVAL '18 days + 2 hours + 34 minutes + 58 seconds',
  NOW() - INTERVAL '18 days + 2 hours + 34 minutes + 58 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000064',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000007',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710040',
  'plink_260710040',
  NULL,
  175,
  275,
  FALSE,
  NOW() - INTERVAL '18 days + 9 hours + 51 minutes + 27 seconds',
  NOW() - INTERVAL '18 days + 9 hours + 51 minutes + 27 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000065',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000008',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710041',
  'plink_260710041',
  NULL,
  200,
  450,
  FALSE,
  NOW() - INTERVAL '17 days + 16 hours + 8 minutes + 56 seconds',
  NOW() - INTERVAL '17 days + 16 hours + 8 minutes + 56 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000066',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000009',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'failed',
  '260710042',
  'plink_260710042',
  'Payment authorization declined by customer issuing bank (Card response: RC_51)',
  225,
  225,
  FALSE,
  NOW() - INTERVAL '17 days + 23 hours + 25 minutes + 25 seconds',
  NOW() - INTERVAL '17 days + 23 hours + 25 minutes + 25 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000067',
  25,
  '11111111-1111-4000-8000-000000000010',
  'System',
  'Behavioral Interview Evaluation (Leadership Principles)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  340,
  315,
  FALSE,
  NOW() - INTERVAL '16 days + 6 hours + 42 minutes + 54 seconds',
  NOW() - INTERVAL '16 days + 6 hours + 42 minutes + 54 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000068',
  15,
  '11111111-1111-4000-8000-000000000011',
  'System',
  'Resume Deep Review & AI Feedback',
  'COIN',
  'failed',
  NULL,
  NULL,
  'Insufficient coin balance during mock interview launch',
  355,
  355,
  FALSE,
  NOW() - INTERVAL '16 days + 13 hours + 59 minutes + 23 seconds',
  NOW() - INTERVAL '16 days + 13 hours + 59 minutes + 23 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000069',
  50,
  '11111111-1111-4000-8000-000000000012',
  'System',
  'Job Posting Feature Boost (7 Days Priority)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  370,
  320,
  FALSE,
  NOW() - INTERVAL '15 days + 20 hours + 16 minutes + 52 seconds',
  NOW() - INTERVAL '15 days + 20 hours + 16 minutes + 52 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000070',
  50000,
  'System',
  '11111111-1111-4000-8000-000000000013',
  'Refund: Cancelled mock interview session (50000 VND)',
  'VND',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  75,
  '22222222-2222-4000-8000-000000000044',
  TRUE,
  FALSE,
  NOW() - INTERVAL '15 days + 3 hours + 33 minutes + 21 seconds',
  NOW() - INTERVAL '15 days + 3 hours + 33 minutes + 21 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000071',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000014',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710043',
  'plink_260710043',
  NULL,
  100,
  350,
  FALSE,
  NOW() - INTERVAL '14 days + 10 hours + 50 minutes + 50 seconds',
  NOW() - INTERVAL '14 days + 10 hours + 50 minutes + 50 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000072',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000015',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'cancelled',
  '260710044',
  'plink_260710044',
  'Customer cancelled transaction on PayOS payment checkout',
  125,
  125,
  FALSE,
  NOW() - INTERVAL '14 days + 17 hours + 7 minutes + 19 seconds',
  NOW() - INTERVAL '14 days + 17 hours + 7 minutes + 19 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000073',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000016',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710045',
  'plink_260710045',
  NULL,
  150,
  250,
  FALSE,
  NOW() - INTERVAL '13 days + 0 hours + 24 minutes + 48 seconds',
  NOW() - INTERVAL '13 days + 0 hours + 24 minutes + 48 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000074',
  250,
  'Banking (PayOS)',
  'a7daf95f-5c68-4ef4-ad62-5166079dbff1',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710046',
  'plink_260710046',
  NULL,
  175,
  425,
  FALSE,
  NOW() - INTERVAL '13 days + 7 hours + 41 minutes + 17 seconds',
  NOW() - INTERVAL '13 days + 7 hours + 41 minutes + 17 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000075',
  600,
  'Banking (PayOS)',
  'c21173fc-ea76-496b-a688-f3aa03f747d5',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710047',
  'plink_260710047',
  NULL,
  200,
  200,
  FALSE,
  NOW() - INTERVAL '13 days + 14 hours + 58 minutes + 46 seconds',
  NOW() - INTERVAL '13 days + 14 hours + 58 minutes + 46 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000076',
  100,
  'Banking (PayOS)',
  '1820709c-d248-4b60-ab6c-ff0cce37508b',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710048',
  'plink_260710048',
  NULL,
  225,
  325,
  FALSE,
  NOW() - INTERVAL '12 days + 21 hours + 15 minutes + 15 seconds',
  NOW() - INTERVAL '12 days + 21 hours + 15 minutes + 15 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000077',
  15,
  '11111111-1111-4000-8000-000000000001',
  'System',
  'Resume Deep Review & AI Feedback',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  490,
  475,
  FALSE,
  NOW() - INTERVAL '12 days + 4 hours + 32 minutes + 44 seconds',
  NOW() - INTERVAL '12 days + 4 hours + 32 minutes + 44 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000078',
  50,
  '11111111-1111-4000-8000-000000000002',
  'System',
  'Job Posting Feature Boost (7 Days Priority)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  505,
  455,
  FALSE,
  NOW() - INTERVAL '12 days + 11 hours + 49 minutes + 13 seconds',
  NOW() - INTERVAL '12 days + 11 hours + 49 minutes + 13 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000079',
  100,
  '11111111-1111-4000-8000-000000000003',
  'System',
  'Recruiter Candidate Assessment Screening Batch',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  520,
  420,
  FALSE,
  NOW() - INTERVAL '11 days + 18 hours + 6 minutes + 42 seconds',
  NOW() - INTERVAL '11 days + 18 hours + 6 minutes + 42 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000080',
  100000,
  'System',
  '11111111-1111-4000-8000-000000000004',
  'Refund: Pro practice package reversal (100000 VND)',
  'VND',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  75,
  '22222222-2222-4000-8000-000000000052',
  TRUE,
  FALSE,
  NOW() - INTERVAL '11 days + 1 hours + 23 minutes + 11 seconds',
  NOW() - INTERVAL '11 days + 1 hours + 23 minutes + 11 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000081',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000005',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710049',
  'plink_260710049',
  NULL,
  100,
  100,
  FALSE,
  NOW() - INTERVAL '11 days + 8 hours + 40 minutes + 40 seconds',
  NOW() - INTERVAL '11 days + 8 hours + 40 minutes + 40 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000082',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000006',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710050',
  'plink_260710050',
  NULL,
  125,
  225,
  FALSE,
  NOW() - INTERVAL '10 days + 15 hours + 57 minutes + 9 seconds',
  NOW() - INTERVAL '10 days + 15 hours + 57 minutes + 9 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000083',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000007',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710051',
  'plink_260710051',
  NULL,
  150,
  400,
  FALSE,
  NOW() - INTERVAL '10 days + 22 hours + 14 minutes + 38 seconds',
  NOW() - INTERVAL '10 days + 22 hours + 14 minutes + 38 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000084',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000008',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'cancelled',
  '260710052',
  'plink_260710052',
  'Customer cancelled transaction on PayOS payment checkout',
  175,
  175,
  FALSE,
  NOW() - INTERVAL '10 days + 5 hours + 31 minutes + 7 seconds',
  NOW() - INTERVAL '10 days + 5 hours + 31 minutes + 7 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000085',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000009',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710053',
  'plink_260710053',
  NULL,
  200,
  300,
  FALSE,
  NOW() - INTERVAL '9 days + 12 hours + 48 minutes + 36 seconds',
  NOW() - INTERVAL '9 days + 12 hours + 48 minutes + 36 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000086',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000010',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710054',
  'plink_260710054',
  NULL,
  225,
  475,
  FALSE,
  NOW() - INTERVAL '9 days + 19 hours + 5 minutes + 5 seconds',
  NOW() - INTERVAL '9 days + 19 hours + 5 minutes + 5 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000087',
  50,
  '11111111-1111-4000-8000-000000000011',
  'System',
  'Job Posting Feature Boost (7 Days Priority)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  340,
  290,
  FALSE,
  NOW() - INTERVAL '9 days + 2 hours + 22 minutes + 34 seconds',
  NOW() - INTERVAL '9 days + 2 hours + 22 minutes + 34 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000088',
  100,
  '11111111-1111-4000-8000-000000000012',
  'System',
  'Recruiter Candidate Assessment Screening Batch',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  355,
  255,
  FALSE,
  NOW() - INTERVAL '8 days + 9 hours + 39 minutes + 3 seconds',
  NOW() - INTERVAL '8 days + 9 hours + 39 minutes + 3 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000089',
  75,
  '11111111-1111-4000-8000-000000000013',
  'System',
  'Technical Evaluation Report Generation',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  370,
  295,
  FALSE,
  NOW() - INTERVAL '8 days + 16 hours + 56 minutes + 32 seconds',
  NOW() - INTERVAL '8 days + 16 hours + 56 minutes + 32 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000090',
  25,
  'System',
  '11111111-1111-4000-8000-000000000014',
  'Refund: System disruption credit compensation',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  100,
  '22222222-2222-4000-8000-000000000056',
  FALSE,
  FALSE,
  NOW() - INTERVAL '8 days + 23 hours + 13 minutes + 1 seconds',
  NOW() - INTERVAL '8 days + 23 hours + 13 minutes + 1 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000091',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000015',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710055',
  'plink_260710055',
  NULL,
  100,
  200,
  FALSE,
  NOW() - INTERVAL '7 days + 6 hours + 30 minutes + 30 seconds',
  NOW() - INTERVAL '7 days + 6 hours + 30 minutes + 30 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000092',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000016',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710056',
  'plink_260710056',
  NULL,
  125,
  375,
  FALSE,
  NOW() - INTERVAL '7 days + 13 hours + 47 minutes + 59 seconds',
  NOW() - INTERVAL '7 days + 13 hours + 47 minutes + 59 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000093',
  600,
  'Banking (PayOS)',
  'a7daf95f-5c68-4ef4-ad62-5166079dbff1',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710057',
  'plink_260710057',
  NULL,
  150,
  150,
  FALSE,
  NOW() - INTERVAL '7 days + 20 hours + 4 minutes + 28 seconds',
  NOW() - INTERVAL '7 days + 20 hours + 4 minutes + 28 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000094',
  100,
  'Banking (PayOS)',
  'c21173fc-ea76-496b-a688-f3aa03f747d5',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710058',
  'plink_260710058',
  NULL,
  175,
  275,
  FALSE,
  NOW() - INTERVAL '6 days + 3 hours + 21 minutes + 57 seconds',
  NOW() - INTERVAL '6 days + 3 hours + 21 minutes + 57 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000095',
  250,
  'Banking (PayOS)',
  '1820709c-d248-4b60-ab6c-ff0cce37508b',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710059',
  'plink_260710059',
  NULL,
  200,
  450,
  FALSE,
  NOW() - INTERVAL '6 days + 10 hours + 38 minutes + 26 seconds',
  NOW() - INTERVAL '6 days + 10 hours + 38 minutes + 26 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000096',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000001',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'cancelled',
  '260710060',
  'plink_260710060',
  'Customer cancelled transaction on PayOS payment checkout',
  225,
  225,
  FALSE,
  NOW() - INTERVAL '6 days + 17 hours + 55 minutes + 55 seconds',
  NOW() - INTERVAL '6 days + 17 hours + 55 minutes + 55 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000097',
  100,
  '11111111-1111-4000-8000-000000000002',
  'System',
  'Recruiter Candidate Assessment Screening Batch',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  490,
  390,
  FALSE,
  NOW() - INTERVAL '5 days + 0 hours + 12 minutes + 24 seconds',
  NOW() - INTERVAL '5 days + 0 hours + 12 minutes + 24 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000098',
  75,
  '11111111-1111-4000-8000-000000000003',
  'System',
  'Technical Evaluation Report Generation',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  505,
  430,
  FALSE,
  NOW() - INTERVAL '5 days + 7 hours + 29 minutes + 53 seconds',
  NOW() - INTERVAL '5 days + 7 hours + 29 minutes + 53 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000099',
  40,
  '11111111-1111-4000-8000-000000000004',
  'System',
  'Live Coding Sandbox Practice Session',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  520,
  480,
  FALSE,
  NOW() - INTERVAL '5 days + 14 hours + 46 minutes + 22 seconds',
  NOW() - INTERVAL '5 days + 14 hours + 46 minutes + 22 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000100',
  50000,
  'System',
  '11111111-1111-4000-8000-000000000005',
  'Refund: Cancelled mock interview session (50000 VND)',
  'VND',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  75,
  '22222222-2222-4000-8000-000000000064',
  TRUE,
  FALSE,
  NOW() - INTERVAL '4 days + 21 hours + 3 minutes + 51 seconds',
  NOW() - INTERVAL '4 days + 21 hours + 3 minutes + 51 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000101',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000006',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710061',
  'plink_260710061',
  NULL,
  100,
  350,
  FALSE,
  NOW() - INTERVAL '4 days + 4 hours + 20 minutes + 20 seconds',
  NOW() - INTERVAL '4 days + 4 hours + 20 minutes + 20 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000102',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000007',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'failed',
  '260710062',
  'plink_260710062',
  'Payment authorization declined by customer issuing bank (Card response: RC_51)',
  125,
  125,
  FALSE,
  NOW() - INTERVAL '4 days + 11 hours + 37 minutes + 49 seconds',
  NOW() - INTERVAL '4 days + 11 hours + 37 minutes + 49 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000103',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000008',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710063',
  'plink_260710063',
  NULL,
  150,
  250,
  FALSE,
  NOW() - INTERVAL '3 days + 18 hours + 54 minutes + 18 seconds',
  NOW() - INTERVAL '3 days + 18 hours + 54 minutes + 18 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000104',
  250,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000009',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710064',
  'plink_260710064',
  NULL,
  175,
  425,
  FALSE,
  NOW() - INTERVAL '3 days + 1 hours + 11 minutes + 47 seconds',
  NOW() - INTERVAL '3 days + 1 hours + 11 minutes + 47 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000105',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000010',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710065',
  'plink_260710065',
  NULL,
  200,
  200,
  FALSE,
  NOW() - INTERVAL '3 days + 8 hours + 28 minutes + 16 seconds',
  NOW() - INTERVAL '3 days + 8 hours + 28 minutes + 16 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000106',
  100,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000011',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710066',
  'plink_260710066',
  NULL,
  225,
  325,
  FALSE,
  NOW() - INTERVAL '2 days + 15 hours + 45 minutes + 45 seconds',
  NOW() - INTERVAL '2 days + 15 hours + 45 minutes + 45 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000107',
  75,
  '11111111-1111-4000-8000-000000000012',
  'System',
  'Technical Evaluation Report Generation',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  340,
  265,
  FALSE,
  NOW() - INTERVAL '2 days + 22 hours + 2 minutes + 14 seconds',
  NOW() - INTERVAL '2 days + 22 hours + 2 minutes + 14 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000108',
  40,
  '11111111-1111-4000-8000-000000000013',
  'System',
  'Live Coding Sandbox Practice Session',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  355,
  315,
  FALSE,
  NOW() - INTERVAL '2 days + 5 hours + 19 minutes + 43 seconds',
  NOW() - INTERVAL '2 days + 5 hours + 19 minutes + 43 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000109',
  20,
  '11111111-1111-4000-8000-000000000014',
  'System',
  'AI Technical Mock Interview Session (Frontend React)',
  'COIN',
  'success',
  NULL,
  NULL,
  NULL,
  370,
  350,
  FALSE,
  NOW() - INTERVAL '1 days + 12 hours + 36 minutes + 12 seconds',
  NOW() - INTERVAL '1 days + 12 hours + 36 minutes + 12 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000110',
  100000,
  'System',
  '11111111-1111-4000-8000-000000000015',
  'Refund: Pro practice package reversal (100000 VND)',
  'VND',
  'success',
  NULL,
  NULL,
  NULL,
  75,
  75,
  '22222222-2222-4000-8000-000000000072',
  TRUE,
  FALSE,
  NOW() - INTERVAL '1 days + 19 hours + 53 minutes + 41 seconds',
  NOW() - INTERVAL '1 days + 19 hours + 53 minutes + 41 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000111',
  600,
  'Banking (PayOS)',
  '11111111-1111-4000-8000-000000000016',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'pending',
  '260710067',
  'plink_260710067',
  NULL,
  100,
  100,
  FALSE,
  NOW() - INTERVAL '1 days + 2 hours + 10 minutes + 10 seconds',
  NOW() - INTERVAL '1 days + 2 hours + 10 minutes + 10 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000112',
  100,
  'Banking (PayOS)',
  'a7daf95f-5c68-4ef4-ad62-5166079dbff1',
  'Deposit: Starter Package (50000 VND)',
  'VND',
  'success',
  '260710068',
  'plink_260710068',
  NULL,
  125,
  225,
  FALSE,
  NOW() - INTERVAL '0 days + 9 hours + 27 minutes + 39 seconds',
  NOW() - INTERVAL '0 days + 9 hours + 27 minutes + 39 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000113',
  250,
  'Banking (PayOS)',
  'c21173fc-ea76-496b-a688-f3aa03f747d5',
  'Deposit: Pro Practice (100000 VND)',
  'VND',
  'success',
  '260710069',
  'plink_260710069',
  NULL,
  150,
  400,
  FALSE,
  NOW() - INTERVAL '0 days + 16 hours + 44 minutes + 8 seconds',
  NOW() - INTERVAL '0 days + 16 hours + 44 minutes + 8 seconds'
);

INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '22222222-2222-4000-8000-000000000114',
  600,
  'Banking (PayOS)',
  '1820709c-d248-4b60-ab6c-ff0cce37508b',
  'Deposit: Enterprise Boost (200000 VND)',
  'VND',
  'failed',
  '260710070',
  'plink_260710070',
  'Payment authorization declined by customer issuing bank (Card response: RC_51)',
  175,
  175,
  FALSE,
  NOW() - INTERVAL '0 days + 23 hours + 1 minutes + 37 seconds',
  NOW() - INTERVAL '0 days + 23 hours + 1 minutes + 37 seconds'
);

-- 4. Initial Billing Audit Logs

INSERT INTO public.billing_audit_logs (
  admin_id, action, transaction_id, reason, metadata, created_at
) VALUES (
  '9def64c6-5cd9-46b0-b1ec-6bfa387f08e1',
  'status_sync',
  '22222222-2222-4000-8000-000000000001',
  'Routine automated verification with PayOS payment gateway',
  '{"gateway":"PayOS","status":"PAID"}'::jsonb,
  NOW() - INTERVAL '5 days'
), (
  '9def64c6-5cd9-46b0-b1ec-6bfa387f08e1',
  'mark_cancelled',
  NULL,
  'Expired pending deposit older than 72 hours cleaned up',
  '{"cleared_orders":2}'::jsonb,
  NOW() - INTERVAL '2 days'
);


COMMIT;