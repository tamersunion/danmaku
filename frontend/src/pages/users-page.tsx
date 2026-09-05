import { useEffect, useState, type ReactElement } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  MoreHorizontalIcon,
  PencilIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  ShieldCheckIcon,
  Trash2Icon,
} from "lucide-react";

import { apiDelete, apiGet, apiPatch, apiPost, apiPut } from "@/api/client";
import type { ApiResponse, ManagedUser } from "@/api/types";
import { useSession } from "@/auth/session-context";
import { ConfirmAction } from "@/components/confirm-action";
import { DataTable, type DataColumn } from "@/components/data-table";
import { ListPagination } from "@/components/list-pagination";
import { LoadingTable } from "@/components/loading-table";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { SearchableSelect } from "@/components/searchable-select";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { useApiMutation } from "@/hooks/use-api-mutation";
import { formatDateTime, initials } from "@/lib/format";

const roleOptions = [
  { value: "user", label: "普通用户" },
  { value: "danmaku_manager", label: "弹幕管理员" },
  { value: "administrator", label: "管理员" },
];
const filterRoles = [{ value: "all", label: "全部角色" }, ...roleOptions];

function managedRoleLabel(role: ManagedUser["role"]): string {
  switch (role) {
    case "administrator":
      return "管理员";
    case "danmaku_manager":
      return "弹幕管理员";
    default:
      return "普通用户";
  }
}

function randomPassword(): string {
  const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789";
  const bytes = crypto.getRandomValues(new Uint8Array(18));
  return Array.from(bytes, (byte) => alphabet[byte % alphabet.length]).join("");
}

export function UsersPage() {
  const session = useSession();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [role, setRole] = useState("all");
  const users = useQuery({
    queryKey: ["users", page, query, role],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<{ total: number; list: ManagedUser[] }>>(
          "/api/admin/users",
          { page, size: 20, q: query, role: role === "all" ? "" : role },
        )
      ).data,
  });
  const status = useApiMutation<ManagedUser, ApiResponse<null>>({
    mutationFn: (user) =>
      apiPatch(`/api/admin/users/${user.id}/status`, {
        enabled: !user.enabled,
      }),
    successMessage: "用户状态已更新",
    invalidate: [["users"]],
  });
  const remove = useApiMutation<number, ApiResponse<null>>({
    mutationFn: (id) => apiDelete(`/api/admin/users/${id}`),
    successMessage: "用户已删除",
    invalidate: [["users"]],
  });
  const columns: DataColumn<ManagedUser>[] = [
    {
      key: "name",
      label: "用户",
      render: (user) => (
        <div className="flex items-center gap-3">
          <Avatar size="sm">
            {user.avatar ? <AvatarImage src={user.avatar} alt="" /> : null}
            <AvatarFallback>{initials(user.displayName)}</AvatarFallback>
          </Avatar>
          <div className="min-w-0">
            <p className="truncate font-medium">{user.displayName}</p>
            <p className="truncate text-xs text-muted-foreground">
              {user.name} · ID {user.id}
            </p>
          </div>
        </div>
      ),
    },
    {
      key: "role",
      label: "角色",
      render: (user) => (
        <Badge
          variant={
            user.role === "administrator"
              ? "default"
              : user.role === "danmaku_manager"
                ? "secondary"
                : "outline"
          }
        >
          {managedRoleLabel(user.role)}
        </Badge>
      ),
    },
    {
      key: "provider",
      label: "来源",
      render: (user) => (
        <Badge variant="outline">
          {user.provider === "cas" ? "CAS" : "本地"}
        </Badge>
      ),
    },
    {
      key: "contact",
      label: "邮箱",
      render: (user) => user.email || "—",
    },
    {
      key: "created",
      label: "创建时间",
      className: "whitespace-nowrap",
      render: (user) => formatDateTime(user.createTime),
    },
    {
      key: "enabled",
      label: "状态",
      render: (user) => (
        <Switch
          aria-label={`${user.enabled ? "停用" : "启用"} ${user.name}`}
          checked={user.enabled}
          disabled={
            status.isPending ||
            user.id === session.id ||
            (user.superAdmin && session.role !== "SuperAdmin")
          }
          onCheckedChange={() => status.mutate(user)}
        />
      ),
    },
    {
      key: "actions",
      label: "",
      className: "w-12",
      render: (user) => (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={`管理 ${user.name}`}
              />
            }
          >
            <MoreHorizontalIcon />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuGroup>
              <UserEditor
                trigger={
                  <DropdownMenuItem closeOnClick={false}>
                    <PencilIcon />
                    编辑
                  </DropdownMenuItem>
                }
                user={user}
              />
              <ConfirmAction
                trigger={
                  <DropdownMenuItem
                    variant="destructive"
                    closeOnClick={false}
                    disabled={
                      user.id === session.id ||
                      (user.superAdmin && session.role !== "SuperAdmin")
                    }
                  >
                    <Trash2Icon />
                    删除
                  </DropdownMenuItem>
                }
                title={`删除用户 ${user.name}？`}
                description={
                  user.provider === "cas"
                    ? "该 CAS 用户下次登录时可能重新自动建档；如需阻止登录，请停用该账户。"
                    : "用户资料将被永久删除。"
                }
                destructive
                pending={remove.isPending}
                onConfirm={() => remove.mutate(user.id)}
              />
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];
  const data = users.data;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Administration"
        title="用户管理"
        description="区分普通用户、弹幕管理员与管理员，并维护账户状态和身份来源。"
        action={
          !session.casEnabled ? (
            <UserEditor
              trigger={
                <Button>
                  <PlusIcon data-icon="inline-start" />
                  添加用户
                </Button>
              }
            />
          ) : null
        }
      />
      {session.casEnabled ? (
        <Alert>
          <ShieldCheckIcon />
          <AlertTitle>用户资料由 CAS 管理</AlertTitle>
          <AlertDescription>
            新用户首次通过 CAS
            登录时自动创建；本页只能调整角色、启停或删除账户，用户名、显示名、邮箱和头像会在每次登录时同步。
          </AlertDescription>
        </Alert>
      ) : null}
      <Card>
        <CardContent className="flex flex-col gap-4 p-0">
          <form
            className="px-6 pt-2"
            onSubmit={(event) => {
              event.preventDefault();
              setPage(1);
              setQuery(search.trim());
            }}
          >
            <FieldGroup className="grid gap-3 md:grid-cols-[minmax(14rem,1fr)_12rem_auto]">
              <Field>
                <FieldLabel className="sr-only" htmlFor="user-search">
                  搜索用户
                </FieldLabel>
                <InputGroup>
                  <InputGroupInput
                    id="user-search"
                    value={search}
                    placeholder="搜索用户名、显示名或邮箱"
                    onChange={(event) => setSearch(event.target.value)}
                  />
                  <InputGroupAddon>
                    <SearchIcon />
                  </InputGroupAddon>
                </InputGroup>
              </Field>
              <Field>
                <FieldLabel className="sr-only" htmlFor="user-role">
                  角色
                </FieldLabel>
                <SearchableSelect
                  id="user-role"
                  options={filterRoles}
                  value={role}
                  onValueChange={(value) => {
                    setRole(value);
                    setPage(1);
                  }}
                  searchPlaceholder="搜索角色"
                />
              </Field>
              <Button type="submit" variant="outline">
                搜索
              </Button>
            </FieldGroup>
          </form>
          {users.isError ? (
            <div className="px-6">
              <QueryError
                error={users.error}
                retry={() => void users.refetch()}
              />
            </div>
          ) : users.isPending ? (
            <LoadingTable />
          ) : (
            <DataTable
              rows={data?.list ?? []}
              columns={columns}
              rowKey={(user) => String(user.id)}
              emptyTitle="暂无用户"
              emptyDescription="CAS 用户会在首次成功登录后显示在这里。"
            />
          )}
        </CardContent>
        {data ? (
          <ListPagination
            meta={{ page, pageSize: 20, total: data.total }}
            onPageChange={setPage}
          />
        ) : null}
      </Card>
    </div>
  );
}

function UserEditor({
  trigger,
  user,
}: {
  trigger: ReactElement;
  user?: ManagedUser;
}) {
  const session = useSession();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<ManagedUser["role"]>("user");
  const detail = useQuery({
    queryKey: ["user", user?.id],
    queryFn: async () =>
      (await apiGet<ApiResponse<ManagedUser>>(`/api/admin/users/${user?.id}`))
        .data,
    enabled: open && Boolean(user),
  });
  const save = useApiMutation<
    Record<string, unknown>,
    ApiResponse<ManagedUser>
  >({
    mutationFn: (body) =>
      user
        ? apiPut(`/api/admin/users/${user.id}`, body)
        : apiPost("/api/admin/users", body),
    successMessage: user ? "用户已更新" : "用户已添加",
    invalidate: [["users"]],
  });

  useEffect(() => {
    if (!open) return;
    const current = detail.data ?? user;
    setName(current?.name ?? "");
    setEmail(current?.email ?? "");
    setRole(current?.role ?? "user");
    setPassword("");
  }, [detail.data, open, user]);

  const current = detail.data ?? user;
  const profileMutable = !user || Boolean(current?.profileMutable);
  const roleLocked = current?.id === session.id;
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={trigger} nativeButton={!user} />
      <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-xl">
        <form
          className="flex flex-col gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            const body: Record<string, unknown> = { role };
            if (profileMutable) {
              body.name = name;
              body.email = email;
              if (password) body.password = password;
            }
            save.mutate(body, { onSuccess: () => setOpen(false) });
          }}
        >
          <DialogHeader>
            <DialogTitle>{user ? "编辑用户" : "添加用户"}</DialogTitle>
            <DialogDescription>
              {profileMutable
                ? "维护本地账户资料、登录密码和角色。"
                : "CAS 负责同步用户资料，本页仅维护系统内角色。"}
            </DialogDescription>
          </DialogHeader>
          {detail.isPending && user ? (
            <LoadingTable rows={5} />
          ) : (
            <FieldGroup>
              {!profileMutable ? (
                <Alert>
                  <ShieldCheckIcon />
                  <AlertTitle>CAS 同步资料</AlertTitle>
                  <AlertDescription>
                    用户名、显示名、邮箱和头像不可在此修改。
                  </AlertDescription>
                </Alert>
              ) : (
                <>
                  <Field>
                    <FieldLabel htmlFor="managed-name">用户名</FieldLabel>
                    <Input
                      id="managed-name"
                      maxLength={16}
                      value={name}
                      required
                      onChange={(event) => setName(event.target.value)}
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="managed-email">邮箱</FieldLabel>
                    <Input
                      id="managed-email"
                      type="email"
                      value={email}
                      onChange={(event) => setEmail(event.target.value)}
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="managed-password">
                      {user ? "重置密码" : "初始密码"}
                    </FieldLabel>
                    <InputGroup>
                      <InputGroupInput
                        id="managed-password"
                        type="password"
                        minLength={6}
                        maxLength={100}
                        required={!user}
                        value={password}
                        onChange={(event) => setPassword(event.target.value)}
                      />
                      <InputGroupAddon align="inline-end">
                        <InputGroupButton
                          type="button"
                          aria-label="生成随机密码"
                          onClick={() => setPassword(randomPassword())}
                        >
                          <RefreshCwIcon />
                        </InputGroupButton>
                      </InputGroupAddon>
                    </InputGroup>
                    <FieldDescription>
                      {user
                        ? "留空表示保持当前密码。"
                        : "至少 6 位，可一键生成随机密码。"}
                    </FieldDescription>
                  </Field>
                </>
              )}
              <Field data-disabled={roleLocked || undefined}>
                <FieldLabel htmlFor="managed-role">角色</FieldLabel>
                <SearchableSelect
                  id="managed-role"
                  options={roleOptions}
                  value={role}
                  disabled={roleLocked}
                  onValueChange={(value) => {
                    if (
                      value === "administrator" ||
                      value === "danmaku_manager" ||
                      value === "user"
                    ) {
                      setRole(value);
                    }
                  }}
                  searchPlaceholder="搜索角色"
                />
                {roleLocked ? (
                  <FieldDescription>
                    为避免锁定管理入口，不能修改自己的角色。
                  </FieldDescription>
                ) : (
                  <FieldDescription>
                    管理员可管理弹幕和用户；弹幕管理员只能管理弹幕；普通用户只能查看个人资料。
                  </FieldDescription>
                )}
              </Field>
            </FieldGroup>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={save.isPending}
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
