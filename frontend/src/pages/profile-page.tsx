import { useEffect, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import md5Module from "js-md5";
import {
  KeyRoundIcon,
  LockKeyholeIcon,
  SaveIcon,
  ShieldCheckIcon,
} from "lucide-react";

import { apiGet, apiPost } from "@/api/client";
import type { ApiResponse, UserProfile } from "@/api/types";
import { useSession } from "@/auth/session-context";
import { LoadingTable } from "@/components/loading-table";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { toast } from "@/components/ui/toast";
import { useApiMutation } from "@/hooks/use-api-mutation";
import { formatDateTime, initials, sessionRoleLabel } from "@/lib/format";

const md5 = md5Module as unknown as (value: string) => string;

export function ProfilePage() {
  const session = useSession();
  const profile = useQuery({
    queryKey: ["profile", session.id],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<UserProfile | null>>("/api/admin/user/user", {
          uid: session.id,
        })
      ).data,
  });
  const user = profile.data;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="平台管理"
        title="个人资料"
        description="查看当前账户、身份来源和可用的安全设置"
      />
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>账户信息</CardTitle>
            <CardDescription>当前会话对应的身份资料</CardDescription>
          </CardHeader>
          <CardContent>
            {profile.isError ? (
              <QueryError
                error={profile.error}
                retry={() => void profile.refetch()}
              />
            ) : profile.isPending ? (
              <LoadingTable rows={3} />
            ) : (
              <div className="flex items-center gap-4">
                <Avatar size="lg">
                  {session.avatar ? (
                    <AvatarImage src={session.avatar} alt="" />
                  ) : null}
                  <AvatarFallback>{initials(session.name)}</AvatarFallback>
                </Avatar>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="truncate font-medium">{session.name}</p>
                    <Badge
                      variant={
                        session.role === "GeneralUser" ? "secondary" : "default"
                      }
                    >
                      {sessionRoleLabel(session.role)}
                    </Badge>
                    <Badge variant="outline">
                      {session.provider === "cas" ? "CAS" : "本地"}
                    </Badge>
                  </div>
                  <p className="truncate text-sm text-muted-foreground">
                    {session.email || session.username}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    创建时间：{formatDateTime(user?.createTime)}
                  </p>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
        {session.casEnabled ? (
          <Card>
            <CardHeader>
              <CardTitle>CAS 同步已启用</CardTitle>
              <CardDescription>
                账户资料和登录凭据由统一身份服务维护
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Alert>
                <ShieldCheckIcon />
                <AlertTitle>本页为只读</AlertTitle>
                <AlertDescription>
                  用户名、显示名、邮箱与头像会在每次 CAS
                  登录时自动同步；密码也不能在本系统修改。如需变更，请前往统一账户中心
                </AlertDescription>
              </Alert>
            </CardContent>
          </Card>
        ) : session.profileEditable ? (
          <Card>
            <CardHeader>
              <CardTitle>账户设置</CardTitle>
              <CardDescription>维护本地资料和登录密码</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              <ProfileEditor profile={user ?? undefined} />
              <PasswordEditor uid={session.id} />
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle>账户设置</CardTitle>
              <CardDescription>
                当前身份来源不允许修改本地资料
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Alert>
                <LockKeyholeIcon />
                <AlertTitle>资料不可修改</AlertTitle>
                <AlertDescription>
                  请联系管理员或身份服务维护账户资料
                </AlertDescription>
              </Alert>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}

function ProfileEditor({ profile }: { profile?: UserProfile }) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const save = useApiMutation<Record<string, unknown>, ApiResponse<null>>({
    mutationFn: (body) => apiPost("/api/admin/user/changeinfo", body),
    successMessage: "个人资料已更新",
    invalidate: [["profile"]],
  });

  useEffect(() => {
    if (!open || !profile) return;
    setName(profile.name);
    setEmail(profile.email ?? "");
    setPhone(profile.phoneNumber ?? "");
  }, [open, profile]);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" disabled={!profile} />}>
        <SaveIcon data-icon="inline-start" />
        编辑资料
      </DialogTrigger>
      <DialogContent>
        <form
          className="flex flex-col gap-5"
          onSubmit={(event: FormEvent) => {
            event.preventDefault();
            if (!profile) return;
            save.mutate(
              { id: profile.id, name, email, phoneNumber: phone },
              { onSuccess: () => setOpen(false) },
            );
          }}
        >
          <DialogHeader>
            <DialogTitle>编辑个人资料</DialogTitle>
            <DialogDescription>
              修改本地账户的名称和联系信息
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="profile-name">用户名</FieldLabel>
              <Input
                id="profile-name"
                maxLength={16}
                value={name}
                required
                onChange={(event) => setName(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="profile-email">邮箱</FieldLabel>
              <Input
                id="profile-email"
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="profile-phone">手机</FieldLabel>
              <Input
                id="profile-phone"
                value={phone}
                onChange={(event) => setPhone(event.target.value)}
              />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
            >
              取消
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? <Spinner data-icon="inline-start" /> : null}保存
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function PasswordEditor({ uid }: { uid: number }) {
  const [open, setOpen] = useState(false);
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const save = useApiMutation<Record<string, unknown>, ApiResponse<null>>({
    mutationFn: (body) => apiPost("/api/admin/user/changepassword", body),
    successMessage: "密码已修改",
  });

  function submit(event: FormEvent) {
    event.preventDefault();
    if (newPassword !== confirmation) {
      toast.add({ title: "两次输入的新密码不一致", type: "error" });
      return;
    }
    if (newPassword === oldPassword) {
      toast.add({ title: "新密码不能与当前密码相同", type: "error" });
      return;
    }
    save.mutate(
      { uid, oldP: md5(oldPassword), newP: md5(newPassword) },
      {
        onSuccess: () => {
          setOpen(false);
          window.location.href = "/api/admin/logout";
        },
      },
    );
  }
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        setOpen(value);
        if (!value) {
          setOldPassword("");
          setNewPassword("");
          setConfirmation("");
        }
      }}
    >
      <DialogTrigger render={<Button variant="outline" />}>
        <KeyRoundIcon data-icon="inline-start" />
        修改密码
      </DialogTrigger>
      <DialogContent>
        <form className="flex flex-col gap-5" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>修改登录密码</DialogTitle>
            <DialogDescription>
              保存成功后会退出当前会话，请使用新密码重新登录
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="current-password">当前密码</FieldLabel>
              <Input
                id="current-password"
                type="password"
                autoComplete="current-password"
                value={oldPassword}
                required
                onChange={(event) => setOldPassword(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="new-password">新密码</FieldLabel>
              <Input
                id="new-password"
                type="password"
                autoComplete="new-password"
                minLength={6}
                value={newPassword}
                required
                onChange={(event) => setNewPassword(event.target.value)}
              />
              <FieldDescription>密码长度不能低于 6 位</FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="confirm-password">确认新密码</FieldLabel>
              <Input
                id="confirm-password"
                type="password"
                autoComplete="new-password"
                minLength={6}
                value={confirmation}
                required
                onChange={(event) => setConfirmation(event.target.value)}
              />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
            >
              取消
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? <Spinner data-icon="inline-start" /> : null}
              确认修改
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
