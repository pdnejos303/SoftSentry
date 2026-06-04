"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Plus } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { useUsers, type DashUser } from "@/lib/users";
import { UserTable } from "@/components/users/UserTable";
import { UserFormDialog } from "@/components/users/UserFormDialog";
import { PasswordRevealDialog } from "@/components/users/PasswordRevealDialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

function useDebounced<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}

const PAGE_SIZE = 50;

export default function UsersPage() {
  const t = useTranslations("users");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  const [q, setQ] = useState("");
  const [role, setRole] = useState("");
  const debouncedQ = useDebounced(q);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<DashUser | null>(null);
  const [reveal, setReveal] = useState<{ password: string; email: string } | null>(null);

  const { data, isLoading } = useUsers({
    page_size: PAGE_SIZE,
    ...(debouncedQ ? { q: debouncedQ } : {}),
    ...(role ? { role } : {}),
  });

  function openAdd() {
    setEditing(null);
    setFormOpen(true);
  }

  function openEdit(u: DashUser) {
    setEditing(u);
    setFormOpen(true);
  }

  if (!isAdmin) {
    return <p className="text-sm text-muted-foreground">{t("adminOnly")}</p>;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Input
          placeholder={t("searchPlaceholder")}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className="max-w-sm"
        />
        <Select value={role} onChange={(e) => setRole(e.target.value)} className="max-w-[160px]">
          <option value="">{t("filter.allRoles")}</option>
          <option value="admin">{t("form.roleAdmin")}</option>
          <option value="viewer">{t("form.roleViewer")}</option>
        </Select>
        <Button className="ml-auto" onClick={openAdd}>
          <Plus className="mr-2 h-4 w-4" />
          {t("addUser")}
        </Button>
      </div>

      <UserTable
        items={data?.items}
        isLoading={isLoading}
        isAdmin={isAdmin}
        currentUserUuid={user?.uuid}
        onEdit={openEdit}
        onPasswordReset={(password, email) => setReveal({ password, email })}
      />

      {data && data.total > 0 && (
        <p className="text-sm text-muted-foreground">{t("totalUsers", { count: data.total })}</p>
      )}

      <UserFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        user={editing}
        onCreated={(password, email) => setReveal({ password, email })}
      />
      <PasswordRevealDialog
        password={reveal?.password ?? null}
        email={reveal?.email}
        onClose={() => setReveal(null)}
      />
    </div>
  );
}
