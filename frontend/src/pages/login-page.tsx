import { useEffect, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import md5Module from "js-md5";
import { KeyRoundIcon, LogInIcon, ShieldCheckIcon } from "lucide-react";

import { apiGet, apiPost, errorMessage } from "@/api/client";
import type { ApiResponse, AuthOptions } from "@/api/types";
import { DanmakuLogo } from "@/components/danmaku-logo";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";

const md5 = md5Module as unknown as (value: string) => string;

function safeRedirect(value: string | null): string {
  return value?.startsWith("/") && !value.startsWith("//") ? value : "/";
}

export function LoginPage() {
  const params = new URLSearchParams(window.location.search);
  const skipSSO = params.get("skipsso") === "true";
  const redirect = safeRedirect(params.get("redirect"));
  const options = useQuery({
    queryKey: ["auth-options"],
    queryFn: async () =>
      (await apiGet<ApiResponse<AuthOptions>>("/api/admin/auth/options")).data,
    retry: false,
  });
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (options.data?.casEnabled && options.data.defaultCAS && !skipSSO) {
      window.location.replace(
        `${options.data.casLoginPath}?returnTo=${encodeURIComponent(redirect)}`,
      );
    }
  }, [options.data, redirect, skipSSO]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setPending(true);
    setError("");
    try {
      await apiPost<ApiResponse<{ url: string; uid: number }>>(
        "/api/admin/login",
        { name, password: md5(password), url: redirect },
      );
      window.location.href = redirect;
    } catch (cause) {
      setError(errorMessage(cause));
      setPending(false);
    }
  }

  const redirecting =
    options.isPending ||
    Boolean(options.data?.casEnabled && options.data.defaultCAS && !skipSSO);
  if (redirecting)
    return (
      <main className="grid min-h-svh place-items-center bg-muted/30 px-4">
        <Card className="w-full max-w-sm">
          <CardHeader>
            <Skeleton className="mx-auto size-12 rounded-xl" />
            <Skeleton className="mx-auto h-6 w-36" />
            <Skeleton className="mx-auto h-4 w-56" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-10 w-full" />
          </CardContent>
        </Card>
      </main>
    );

  return (
    <main className="grid min-h-svh place-items-center bg-muted/30 px-4 py-10">
      <Card className="w-full max-w-md shadow-lg shadow-black/5">
        <CardHeader className="text-center">
          <span className="mx-auto flex size-12 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm">
            <DanmakuLogo className="size-8" />
          </span>
          <CardTitle>弹幕控制台</CardTitle>
          <CardDescription>登录后管理弹幕数据与用户权限</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="flex flex-col gap-5" onSubmit={submit}>
            {error ? (
              <Alert variant="destructive">
                <AlertTitle>登录失败</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="login-name">用户名</FieldLabel>
                <Input
                  id="login-name"
                  autoComplete="username"
                  value={name}
                  required
                  onChange={(event) => setName(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="login-password">密码</FieldLabel>
                <Input
                  id="login-password"
                  type="password"
                  autoComplete="current-password"
                  minLength={6}
                  value={password}
                  required
                  onChange={(event) => setPassword(event.target.value)}
                />
                <FieldDescription>使用本地账号密码登录</FieldDescription>
              </Field>
            </FieldGroup>
            <Button type="submit" disabled={pending}>
              {pending ? (
                <Spinner data-icon="inline-start" />
              ) : (
                <LogInIcon data-icon="inline-start" />
              )}
              登录
            </Button>
          </form>
        </CardContent>
        {options.data?.casEnabled ? (
          <CardFooter className="flex flex-col gap-3">
            <Button
              className="w-full"
              variant="outline"
              onClick={() => {
                window.location.href = `${options.data.casLoginPath}?returnTo=${encodeURIComponent(redirect)}`;
              }}
            >
              <ShieldCheckIcon data-icon="inline-start" />
              使用 CAS 登录
            </Button>
            <p className="text-center text-xs text-muted-foreground">
              <KeyRoundIcon className="mr-1 inline" />
              本地登录仅用于应急访问
            </p>
          </CardFooter>
        ) : null}
      </Card>
    </main>
  );
}
