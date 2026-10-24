// scripts/seed_billing_90days.js
// Seeds realistic Candidate/Recruiter users, wallets, and 90-day transactions into RoleCue Postgres.
// Compatible with Node.js built-ins (no external npm dependencies required).

const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");

const USERS = [
  {
    id: "11111111-1111-4000-8000-000000000001",
    name: "Nguyễn Hoàng Nam",
    email: "nam.nguyen@gmail.com",
    role: "candidate",
    daysAgo: 88,
  },
  {
    id: "11111111-1111-4000-8000-000000000002",
    name: "Trần Thị Minh Anh",
    email: "minhanh.tran@outlook.com",
    role: "candidate",
    daysAgo: 84,
  },
  {
    id: "11111111-1111-4000-8000-000000000003",
    name: "Lê Quốc Bảo",
    email: "bao.le@techdev.io",
    role: "candidate",
    daysAgo: 79,
  },
  {
    id: "11111111-1111-4000-8000-000000000004",
    name: "Phan Bích Ngọc",
    email: "ngoc.phan@fpt.com",
    role: "recruiter",
    company_name: "FPT Software",
    company_website: "https://fpt-software.com",
    daysAgo: 75,
  },
  {
    id: "11111111-1111-4000-8000-000000000005",
    name: "Phạm Thu Hương",
    email: "huong.pham@fpt.edu.vn",
    role: "candidate",
    daysAgo: 69,
  },
  {
    id: "11111111-1111-4000-8000-000000000006",
    name: "Trương Hữu Nghĩa",
    email: "nghia.truong@viettel.vn",
    role: "recruiter",
    company_name: "Viettel Solutions",
    company_website: "https://solutions.viettel.vn",
    daysAgo: 64,
  },
  {
    id: "11111111-1111-4000-8000-000000000007",
    name: "Võ Đức Trí",
    email: "tri.vo@gmail.com",
    role: "candidate",
    daysAgo: 58,
  },
  {
    id: "11111111-1111-4000-8000-000000000008",
    name: "Alex Johnson",
    email: "alex.j@vng.com.vn",
    role: "recruiter",
    company_name: "VNG Corporation",
    company_website: "https://vng.com.vn",
    daysAgo: 53,
  },
  {
    id: "11111111-1111-4000-8000-000000000009",
    name: "Đỗ Mỹ Linh",
    email: "linh.do@hust.edu.vn",
    role: "candidate",
    daysAgo: 47,
  },
  {
    id: "11111111-1111-4000-8000-000000000010",
    name: "Bùi Văn Thành",
    email: "thanh.bui@vnu.edu.vn",
    role: "candidate",
    daysAgo: 41,
  },
  {
    id: "11111111-1111-4000-8000-000000000011",
    name: "Elena Rostova",
    email: "elena@tiki.vn",
    role: "recruiter",
    company_name: "Tiki Corp",
    company_website: "https://tiki.vn",
    daysAgo: 35,
  },
  {
    id: "11111111-1111-4000-8000-000000000012",
    name: "Hoàng Yến Nhi",
    email: "nhi.hoang@gmail.com",
    role: "candidate",
    daysAgo: 29,
  },
  {
    id: "11111111-1111-4000-8000-000000000013",
    name: "Vũ Minh Quân",
    email: "quan.vu@onemount.com",
    role: "recruiter",
    company_name: "One Mount Group",
    company_website: "https://onemount.com",
    daysAgo: 21,
  },
  {
    id: "11111111-1111-4000-8000-000000000014",
    name: "Ngô Gia Huy",
    email: "huy.ngo@uit.edu.vn",
    role: "candidate",
    daysAgo: 15,
  },
  {
    id: "11111111-1111-4000-8000-000000000015",
    name: "Đặng Phương Thảo",
    email: "thao.dang@gmail.com",
    role: "candidate",
    daysAgo: 9,
  },
  {
    id: "11111111-1111-4000-8000-000000000016",
    name: "Nguyễn Hải Đăng",
    email: "dang.nguyen@gmail.com",
    role: "candidate",
    daysAgo: 3,
  },
];

// Include existing users for additional activity
const ALL_ACTIVE_USER_IDS = [
  ...USERS.map((u) => u.id),
  "a7daf95f-5c68-4ef4-ad62-5166079dbff1", // Sample Candidate
  "c21173fc-ea76-496b-a688-f3aa03f747d5", // Sample Recruiter
  "1820709c-d248-4b60-ab6c-ff0cce37508b", // Nhật Anh
];

const PACKAGES = [
  { name: "Starter Package", coins: 100, price: 50000 },
  { name: "Pro Practice", coins: 250, price: 100000 },
  { name: "Enterprise Boost", coins: 600, price: 200000 },
];

const SPEND_ACTIVITIES = [
  { amount: 20, desc: "AI Technical Mock Interview Session (Frontend React)" },
  { amount: 25, desc: "AI Technical Mock Interview Session (Golang Backend)" },
  { amount: 30, desc: "System Design Mock Interview (Distributed Systems)" },
  { amount: 25, desc: "Behavioral Interview Evaluation (Leadership Principles)" },
  { amount: 15, desc: "Resume Deep Review & AI Feedback" },
  { amount: 50, desc: "Job Posting Feature Boost (7 Days Priority)" },
  { amount: 100, desc: "Recruiter Candidate Assessment Screening Batch" },
  { amount: 75, desc: "Technical Evaluation Report Generation" },
  { amount: 40, desc: "Live Coding Sandbox Practice Session" },
];

const REFUND_ACTIVITIES = [
  { amount: 50000, currency: "VND", desc: "Refund: Cancelled mock interview session (50000 VND)", clawback: true },
  { amount: 100000, currency: "VND", desc: "Refund: Pro practice package reversal (100000 VND)", clawback: true },
  { amount: 25, currency: "COIN", desc: "Refund: System disruption credit compensation", clawback: false },
];

function generateSql() {
  const statements = [];

  statements.push("-- =============================================================================");
  statements.push("-- RoleCue Billing Seed Data: Users, Wallets & 90-Day Transactions");
  statements.push("-- Generated: " + new Date().toISOString());
  statements.push("-- =============================================================================");
  statements.push("BEGIN;\n");

  // Clean up any prior seeded 90-day transactions to allow safe re-running
  statements.push("-- 0. Clean prior seed transactions (identified by order code prefix '26071' or seed descriptions)");
  statements.push(`
DELETE FROM public.transactions
WHERE (payos_order_code IS NOT NULL AND payos_order_code LIKE '26071%')
   OR description LIKE '%(Frontend React)%'
   OR description LIKE '%(Golang Backend)%'
   OR description LIKE '%(Distributed Systems)%'
   OR description LIKE '%(Leadership Principles)%'
   OR description LIKE '%package reversal%'
   OR description LIKE '%credit compensation%';
`);

  // 1. Insert Users
  statements.push("-- 1. Insert Seed Users");
  for (const u of USERS) {
    const company = u.company_name ? `'${u.company_name}'` : "NULL";
    const website = u.company_website ? `'${u.company_website}'` : "NULL";
    statements.push(`
INSERT INTO public.users (
  id, name, email, email_verified, role, onboarding_status,
  company_name, company_website, created_at, updated_at
) VALUES (
  '${u.id}',
  '${u.name.replace(/'/g, "''")}',
  '${u.email}',
  true,
  '${u.role}',
  'completed',
  ${company},
  ${website},
  NOW() - INTERVAL '${u.daysAgo} days',
  NOW() - INTERVAL '${u.daysAgo} days'
) ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  company_name = EXCLUDED.company_name,
  company_website = EXCLUDED.company_website;
`);
  }

  // 2. Insert Wallets for Users
  statements.push("\n-- 2. Insert Wallets for Seed Users");
  for (let i = 0; i < USERS.length; i++) {
    const u = USERS[i];
    const initialBalance = ((i % 4) + 1) * 75; // 75, 150, 225, 300
    statements.push(`
INSERT INTO public.wallets (id, user_id, balance)
VALUES (
  gen_random_uuid(),
  '${u.id}',
  ${initialBalance}
) ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;
`);
  }

  // 3. Transactions across the last 90 days
  statements.push("\n-- 3. Seed Transactions Across Last 90 Days");

  let orderCodeSeq = 260710000;
  let txIndex = 0;

  // We distribute ~114 transactions over the 90 days.
  // More recent days have higher frequency.
  const daysList = [];
  for (let d = 88; d >= 0; d--) {
    const freq = d < 14 ? 3 : d < 30 ? 2 : d < 60 ? 1 : d % 3 === 0 ? 1 : 0;
    for (let f = 0; f < freq; f++) {
      daysList.push(d);
    }
  }

  let pendingCount = 0;
  let failedCount = 0;
  let cancelledCount = 0;
  let successCount = 0;

  // Keep track of earlier deposit UUIDs to link refunds
  const topupIds = [];

  for (let idx = 0; idx < daysList.length; idx++) {
    const day = daysList[idx];
    const user = ALL_ACTIVE_USER_IDS[idx % ALL_ACTIVE_USER_IDS.length];
    const hour = (idx * 7) % 24;
    const min = (idx * 17) % 60;
    const sec = (idx * 29) % 60;
    const intervalStr = `${day} days + ${hour} hours + ${min} minutes + ${sec} seconds`;
    const txId = `22222222-2222-4000-8000-${String(idx + 1).padStart(12, "0")}`;

    // Rotate transaction types:
    // ~55% Top-up (deposit)
    // ~35% Spend
    // ~10% Refund
    const txTypeMod = idx % 10;

    if (txTypeMod <= 5) {
      // TOP-UP / DEPOSIT
      const pkg = PACKAGES[idx % PACKAGES.length];
      topupIds.push(txId);
      
      // Determine realistic status distribution across top-ups
      const statusMod = idx % 12;
      let status = "success";
      let failureReason = "NULL";
      if (statusMod === 2 || statusMod === 8) {
        status = "pending";
        pendingCount++;
      } else if (statusMod === 5) {
        status = "failed";
        failureReason = `'Payment authorization declined by customer issuing bank (Card response: RC_51)'`;
        failedCount++;
      } else if (statusMod === 11) {
        status = "cancelled";
        failureReason = `'Customer cancelled transaction on PayOS payment checkout'`;
        cancelledCount++;
      } else {
        successCount++;
      }

      orderCodeSeq++;
      const orderCode = `'${orderCodeSeq}'`;
      const desc = `Deposit: ${pkg.name} (${pkg.price} VND)`;
      const plink = `'plink_${orderCodeSeq}'`;
      const balBefore = 100 + ((idx * 25) % 250);
      const balAfter = status === "success" ? balBefore + pkg.coins : balBefore;
      const isTestRow = idx === 0 ? "TRUE" : "FALSE"; // mark 1 as explicit test for demo toggle

      statements.push(`
INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '${txId}',
  ${pkg.coins},
  'Banking (PayOS)',
  '${user}',
  '${desc}',
  'VND',
  '${status}',
  ${orderCode},
  ${plink},
  ${failureReason},
  ${balBefore},
  ${balAfter},
  ${isTestRow},
  NOW() - INTERVAL '${intervalStr}',
  NOW() - INTERVAL '${intervalStr}'
);`);
      txIndex++;
    } else if (txTypeMod >= 6 && txTypeMod <= 8) {
      // SPEND
      const activity = SPEND_ACTIVITIES[idx % SPEND_ACTIVITIES.length];
      const isSpendFailed = idx === 17 || idx === 67;
      const status = isSpendFailed ? "failed" : "success";
      const failureReason = isSpendFailed ? `'Insufficient coin balance during mock interview launch'` : "NULL";
      if (isSpendFailed) failedCount++;
      else successCount++;

      const balBefore = 250 + ((idx * 15) % 300);
      const balAfter = status === "success" ? balBefore - activity.amount : balBefore;

      statements.push(`
INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after, is_test,
  created_at, updated_at
) VALUES (
  '${txId}',
  ${activity.amount},
  '${user}',
  'System',
  '${activity.desc}',
  'COIN',
  '${status}',
  NULL,
  NULL,
  ${failureReason},
  ${balBefore},
  ${balAfter},
  FALSE,
  NOW() - INTERVAL '${intervalStr}',
  NOW() - INTERVAL '${intervalStr}'
);`);
      txIndex++;
    } else {
      // REFUND
      const refund = REFUND_ACTIVITIES[idx % REFUND_ACTIVITIES.length];
      const status = "success";
      successCount++;

      // Link to an earlier deposit if available
      const origId = topupIds.length > 0 ? `'${topupIds[idx % topupIds.length]}'` : "NULL";
      const clawback = refund.clawback ? "TRUE" : "FALSE";
      const balBefore = 75;
      const balAfter = refund.currency === "COIN" ? 100 : 75;

      statements.push(`
INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  payos_payment_link_id, failure_reason, balance_before, balance_after,
  original_transaction_id, coin_clawed_back, is_test,
  created_at, updated_at
) VALUES (
  '${txId}',
  ${refund.amount},
  'System',
  '${user}',
  '${refund.desc}',
  '${refund.currency}',
  '${status}',
  NULL,
  NULL,
  NULL,
  ${balBefore},
  ${balAfter},
  ${origId},
  ${clawback},
  FALSE,
  NOW() - INTERVAL '${intervalStr}',
  NOW() - INTERVAL '${intervalStr}'
);`);
      txIndex++;
    }
  }

  // 4. Initial audit logs
  statements.push("\n-- 4. Initial Billing Audit Logs");
  statements.push(`
INSERT INTO public.billing_audit_logs (
  admin_id, action, transaction_id, reason, metadata, created_at
) VALUES (
  '9def64c6-5cd9-46b0-b1ec-6bfa387f08e1',
  'status_sync',
  '${topupIds[0]}',
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
`);

  statements.push("\nCOMMIT;");
  return {
    sql: statements.join("\n"),
    txCount: txIndex,
    stats: { successCount, pendingCount, failedCount, cancelledCount },
  };
}

function main() {
  const sqlPath = path.join(__dirname, "seed_billing_90days.sql");
  console.log(`Generating 90-day seed data SQL...`);
  const { sql, txCount, stats } = generateSql();
  fs.writeFileSync(sqlPath, sql, "utf-8");
  console.log(
    `Generated ${txCount} transactions (${stats.successCount} success, ${stats.pendingCount} pending, ${stats.failedCount} failed, ${stats.cancelledCount} cancelled) and ${USERS.length} users into: ${sqlPath}`
  );

  console.log("Applying to PostgreSQL container 'rolecue-postgres'...");
  try {
    execSync(`docker exec -i rolecue-postgres psql -U postgres -d rolecue -f - < "${sqlPath}"`, {
      stdio: "inherit",
      shell: "cmd.exe",
    });
    console.log("Successfully seeded 90 days of accounts and transactions!");
  } catch (err) {
    console.error("Failed to run psql via Docker:", err.message);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}
