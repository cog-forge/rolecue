"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { authClient } from "@/lib/auth/client";
import { Button } from "@/components/ui/button";
import { AccountLockAction } from "./account-lock-action";

export function AccountManagement() {
  const [offset, setOffset] = useState(0);
  const limit = 20;
  const users = useQuery({
    queryKey: ["auth", "admin-users", offset],
    queryFn: async () => {
      const result = await authClient.admin.listUsers({
        query: { limit, offset, sortBy: "createdAt", sortDirection: "desc" },
      });
      if (result.error || !result.data) throw new Error("Unable to load users");
      return result.data;
    },
    staleTime: 0,
  });
  if (users.isPending) return <p role="status">Loading users…</p>;
  if (users.error)
    return (
      <div role="alert">
        <p>Unable to load users.</p>
        <Button onClick={() => void users.refetch()}>Retry</Button>
      </div>
    );
  return (
    <section className="space-y-4" aria-label="Account management">
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead>
            <tr>
              <th className="p-3">Email</th>
              <th className="p-3">Role</th>
              <th className="p-3">Status</th>
              <th className="p-3">Action</th>
            </tr>
          </thead>
          <tbody>
            {users.data.users.map((user) => (
              <tr key={user.id} className="border-t">
                <td className="p-3">{user.email}</td>
                <td className="p-3">{user.role}</td>
                <td className="p-3">{user.banned ? "Locked" : "Active"}</td>
                <td className="p-3">
                  <AccountLockAction
                    userId={user.id}
                    locked={Boolean(user.banned)}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="flex items-center gap-4">
        <Button
          variant="outline"
          disabled={offset === 0}
          onClick={() => setOffset((value) => Math.max(0, value - limit))}
        >
          Previous
        </Button>
        <span>Page {offset / limit + 1}</span>
        <Button
          variant="outline"
          disabled={offset + limit >= users.data.total}
          onClick={() => setOffset((value) => value + limit)}
        >
          Next
        </Button>
      </div>
    </section>
  );
}
