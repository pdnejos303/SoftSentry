"use client";

import { useState } from "react";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { useRouter } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import { toast } from "sonner";
import axios from "axios";
import { ArrowRight, Check, Eye, EyeOff, Lock, Mail } from "lucide-react";

export default function LoginPage() {
  const t = useTranslations("login");
  const { login } = useAuth();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
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
    <main className="relative flex min-h-screen w-full items-center justify-center bg-[#f4f0e8] p-4 sm:p-8">
      <div className="w-full max-w-6xl">
        {/* logo + language switcher — outside the card */}
        <div className="mb-6 flex items-center justify-between px-1">
          <Image
            src="/static/img/logo_runexy.png"
            alt="Runexy"
            width={220}
            height={56}
            priority
            className="h-12 w-auto object-contain"
          />
          <LanguageSwitcher />
        </div>

        {/* ---------- Card ---------- */}
        <div className="grid grid-cols-1 overflow-hidden rounded-[28px] bg-[#faf7f1] shadow-[0_30px_80px_-20px_rgba(60,40,20,0.25)] ring-1 ring-black/5 md:min-h-[650px] md:grid-cols-2">
          {/* ---------- Left brand pane ---------- */}
          <aside className="relative hidden flex-col justify-between gap-10 p-10 md:flex">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.25em] text-[#a8987f]">
                — SoftSentry
              </p>
              <h2 className="mt-8 font-serif text-4xl font-medium leading-[1.2] text-[#2c2419] lg:text-5xl">
                {t("brandHeadline")}
              </h2>
              <p className="mt-5 max-w-xs text-sm leading-relaxed text-[#8a7c66]">
                {t("brandTagline")}
              </p>
            </div>

            {/* feature highlights */}
            <ul className="space-y-3.5">
              {(t.raw("features") as string[]).map((feature, i) => (
                <li key={i} className="flex items-center gap-3 text-sm text-[#6b5d49]">
                  <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#c0392b]/10 text-[#c0392b]">
                    <Check className="h-3 w-3" strokeWidth={3} />
                  </span>
                  {feature}
                </li>
              ))}
            </ul>

            {/* software asset management illustration: inventory dashboard + security shield */}
            <div className="flex items-center justify-center">
              <svg
                viewBox="0 0 220 150"
                className="h-52 w-auto"
                fill="none"
                aria-hidden="true"
              >
                {/* dashboard / inventory panel */}
                <rect x="22" y="24" width="132" height="94" rx="10" fill="#ffffff" stroke="#e2d8c6" strokeWidth="2" />
                {/* window dots */}
                <circle cx="36" cy="38" r="2.6" fill="#c0392b" />
                <circle cx="46" cy="38" r="2.6" fill="#d9b08c" />
                <circle cx="56" cy="38" r="2.6" fill="#cbb89a" />
                <line x1="30" y1="50" x2="146" y2="50" stroke="#efe6d6" strokeWidth="2" />
                {/* inventory bars */}
                <rect x="40" y="80" width="14" height="26" rx="3" fill="#cbb89a" />
                <rect x="62" y="66" width="14" height="40" rx="3" fill="#6b5d49" />
                <rect x="84" y="86" width="14" height="20" rx="3" fill="#d9c4a8" />
                <rect x="106" y="58" width="14" height="48" rx="3" fill="#c0392b" />
                {/* security shield with check */}
                <path
                  d="M168 70 L190 78 L190 98 Q190 116 168 126 Q146 116 146 98 L146 78 Z"
                  fill="#2c2419"
                  stroke="#faf7f1"
                  strokeWidth="3"
                />
                <path
                  d="M159 98 l6 6 l12 -14"
                  stroke="#faf7f1"
                  strokeWidth="3.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </div>
          </aside>

          {/* ---------- Right form pane ---------- */}
          <section className="flex flex-col justify-center border-[#e7ddcc] bg-white p-8 sm:p-12 md:border-l">
            <div className="mx-auto w-full max-w-sm">
              <span className="inline-flex items-center gap-1.5 rounded-full border border-[#e6c9b8] bg-[#fbf2ec] px-3 py-1 text-xs font-medium text-[#c0392b]">
                <span className="h-1.5 w-1.5 rounded-full bg-[#c0392b]" />
                {t("secure")}
              </span>

              <h1 className="mt-5 font-serif text-3xl font-medium tracking-tight text-[#2c2419]">
                {t("welcome")}
              </h1>
              <p className="mt-2 text-sm text-[#8a7c66]">{t("subtitle")}</p>

              <form onSubmit={onSubmit} className="mt-7 space-y-5">
                {/* email */}
                <div className="space-y-2">
                  <Label
                    htmlFor="email"
                    className="text-[11px] font-semibold uppercase tracking-wide text-[#a8987f]"
                  >
                    {t("email")}
                  </Label>
                  <div className="relative">
                    <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#b8aa92]" />
                    <Input
                      id="email"
                      type="email"
                      required
                      autoComplete="email"
                      placeholder="you@company.com"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      className="h-11 border-[#e2d8c6] bg-[#fcfaf6] pl-10"
                    />
                  </div>
                </div>

                {/* password */}
                <div className="space-y-2">
                  <Label
                    htmlFor="password"
                    className="text-[11px] font-semibold uppercase tracking-wide text-[#a8987f]"
                  >
                    {t("password")}
                  </Label>
                  <div className="relative">
                    <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#b8aa92]" />
                    <Input
                      id="password"
                      type={showPassword ? "text" : "password"}
                      required
                      autoComplete="current-password"
                      placeholder="••••••••"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      className="h-11 border-[#e2d8c6] bg-[#fcfaf6] pl-10 pr-10"
                    />
                    <button
                      type="button"
                      aria-label={showPassword ? "Hide password" : "Show password"}
                      onClick={() => setShowPassword((v) => !v)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-[#b8aa92] hover:text-[#6b5d49]"
                    >
                      {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                </div>

                {/* remember me */}
                <label className="flex items-center gap-2 text-sm text-[#6b5d49]">
                  <input
                    type="checkbox"
                    checked={rememberMe}
                    onChange={(e) => setRememberMe(e.target.checked)}
                    className="h-4 w-4 rounded border-[#cabba2] accent-[#2c2419]"
                  />
                  {t("rememberMe")}
                </label>

                {/* submit */}
                <Button
                  type="submit"
                  disabled={submitting}
                  className="h-12 w-full gap-2 rounded-xl bg-[#BF2002] text-sm tracking-wide text-white hover:bg-[#332c20]"
                >
                  {submitting ? t("submitting") : t("submit")}
                  {!submitting && <ArrowRight className="h-4 w-4" />}
                </Button>
              </form>
            </div>
          </section>
        </div>
      </div>
    </main>
  );
}
