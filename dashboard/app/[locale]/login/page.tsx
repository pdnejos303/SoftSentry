"use client";

import { useState } from "react";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { useRouter } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import { toast } from "sonner";
import axios from "axios";

export default function LoginPage() {
  const t = useTranslations("login");
  const { login } = useAuth();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [rememberMe, setRememberMe] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      await login(email, password, rememberMe);
      router.push("/");
    } catch (err) {
      const msg =
        axios.isAxiosError(err) && err.response?.status === 429
          ? t("errors.locked")
          : axios.isAxiosError(err) && err.response?.status === 403
            ? t("errors.inactive")
            : axios.isAxiosError(err) && err.response?.status === 401
              ? t("errors.invalid")
              : t("errors.generic");
      toast.error(msg);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 bg-slate-100 p-4">
      <h1 className="text-4xl font-bold text-red-800">{t("heading")}</h1>
      <Card className="relative grid w-full max-w-3xl rounded-2xl overflow-hidden p-0 shadow-2xl md:min-h-[500px] md:grid-cols-[2fr_3fr]">
        <div className="absolute right-4 top-4 z-10">
          <LanguageSwitcher />
        </div>

        <div className="relative bg-orange-200 hidden md:block">
          <Image
            src="/static/img/logo_runexy.png"
            alt="Runexy Logo"
            fill
            priority
            className="object-contain p-6"
          />
        </div>

        <div className="flex flex-col justify-center p-8">
          <CardHeader className="px-0">
            <CardTitle>{t("title")}</CardTitle>
            <CardDescription>{t("subtitle")}</CardDescription>
          </CardHeader>
          <CardContent className="px-0">
            <form onSubmit={onSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email">{t("email")}</Label>
                <Input
                  id="email"
                  type="email"
                  required
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">{t("password")}</Label>
                <Input
                  id="password"
                  type="password"
                  required
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={rememberMe}
                  onChange={(e) => setRememberMe(e.target.checked)}
                  className="h-4 w-4 rounded border-input"
                />
                {t("rememberMe")}
              </label>
              <Button type="submit" disabled={submitting} className="w-full">
                {submitting ? t("submitting") : t("submit")}
              </Button>
            </form>
          </CardContent>
        </div>
      </Card>
    </main>
  );
}
