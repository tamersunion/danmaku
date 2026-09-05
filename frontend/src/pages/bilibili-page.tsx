import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  EyeIcon,
  LinkIcon,
  PencilIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  ShieldBanIcon,
  Trash2Icon,
} from "lucide-react";

import { apiDelete, apiGet, apiPatch, apiPost } from "@/api/client";
import type {
  ApiResponse,
  BilibiliBinding,
  BilibiliKeyword,
  BilibiliPool,
  BilibiliPoolDanmaku,
} from "@/api/types";
import { ConfirmAction } from "@/components/confirm-action";
import { DataTable, type DataColumn } from "@/components/data-table";
import { ListPagination } from "@/components/list-pagination";
import { LoadingTable } from "@/components/loading-table";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardAction,
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
} from "@/components/ui/dialog";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { useApiMutation } from "@/hooks/use-api-mutation";
import { formatDateTime } from "@/lib/format";

type Paged<T> = { total: number; list: T[] };
type PoolMutationResult = { pool: BilibiliPool; inserted: number };
type PoolIdentifierType = "bvid" | "aid" | "cid";
type CreatePoolInput = {
  bvid?: string;
  aid?: number;
  cid?: number;
  p: number;
};

function poolLabel(pool: Pick<BilibiliPool, "bvid" | "cid" | "p">): string {
  if (!pool.bvid) return `CID ${pool.cid} / P${pool.p}`;
  return `${pool.bvid} / P${pool.p}`;
}

function keywordPoolLabel(keyword: BilibiliKeyword): string {
  if (!keyword.poolBvid) return `CID ${keyword.poolCid} / P${keyword.poolP}`;
  return `${keyword.poolBvid} / P${keyword.poolP}`;
}

function offsetLabel(offset: number): string {
  if (offset > 0) return `+${offset} 秒`;
  return `${offset} 秒`;
}

export function BilibiliPage() {
  const poolOptions = useQuery({
    queryKey: ["bilibili-pool-options"],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<BilibiliPool>>>(
          "/api/admin/bilibili/pools",
          { page: 1, size: 500 },
        )
      ).data.list,
  });

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Library"
        title="Bilibili 弹幕库"
        description="持久化管理 Bilibili 弹幕池、过滤规则以及与本地视频 ID 的关联。"
      />
      <Tabs defaultValue="pools">
        <TabsList>
          <TabsTrigger value="pools">弹幕池</TabsTrigger>
          <TabsTrigger value="keywords">过滤关键词</TabsTrigger>
          <TabsTrigger value="bindings">关联设置</TabsTrigger>
        </TabsList>
        <TabsContent value="pools" className="flex flex-col gap-4">
          <PoolPanel />
        </TabsContent>
        <TabsContent value="keywords" className="flex flex-col gap-4">
          <KeywordPanel pools={poolOptions.data ?? []} />
        </TabsContent>
        <TabsContent value="bindings" className="flex flex-col gap-4">
          <BindingPanel pools={poolOptions.data ?? []} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function PoolPanel() {
  const [page, setPage] = useState(1);
  const [draftQuery, setDraftQuery] = useState("");
  const [query, setQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [identifierType, setIdentifierType] =
    useState<PoolIdentifierType>("bvid");
  const [identifier, setIdentifier] = useState("");
  const [part, setPart] = useState("1");
  const [selectedPool, setSelectedPool] = useState<BilibiliPool | null>(null);
  const pools = useQuery({
    queryKey: ["bilibili-pools", page, query],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<BilibiliPool>>>(
          "/api/admin/bilibili/pools",
          { page, size: 20, query },
        )
      ).data,
  });
  const create = useApiMutation<
    CreatePoolInput,
    ApiResponse<PoolMutationResult>
  >({
    mutationFn: (body) => apiPost("/api/admin/bilibili/pools", body),
    successMessage: "弹幕池已创建并同步",
    invalidate: [["bilibili-pools"], ["bilibili-pool-options"]],
  });
  const sync = useApiMutation<number, ApiResponse<PoolMutationResult>>({
    mutationFn: (id) => apiPost(`/api/admin/bilibili/pools/${id}/sync`),
    successMessage: "弹幕池已增量同步",
    invalidate: [
      ["bilibili-pools"],
      ["bilibili-pool-options"],
      ["bilibili-pool-danmaku"],
    ],
  });
  const columns = useMemo<DataColumn<BilibiliPool>[]>(
    () => [
      {
        key: "pool",
        label: "弹幕池",
        render: (pool) => (
          <div>
            <p className="font-medium">{poolLabel(pool)}</p>
            <p className="font-mono text-xs text-muted-foreground">
              CID {pool.cid}
            </p>
          </div>
        ),
      },
      {
        key: "data",
        label: "弹幕",
        render: (pool) => (
          <div className="flex flex-wrap gap-2">
            <Badge variant="secondary">共 {pool.danmakuCount} 条</Badge>
            <Badge variant={pool.blockedCount ? "destructive" : "outline"}>
              屏蔽 {pool.blockedCount} 条
            </Badge>
          </div>
        ),
      },
      {
        key: "bindings",
        label: "关联",
        render: (pool) => `${pool.bindingCount} 个视频 ID`,
      },
      {
        key: "sync",
        label: "最后同步",
        className: "whitespace-nowrap",
        render: (pool) =>
          pool.lastSyncTime ? formatDateTime(pool.lastSyncTime) : "尚未同步",
      },
      {
        key: "actions",
        label: "操作",
        className: "w-24 whitespace-nowrap text-right",
        render: (pool) => (
          <div className="flex justify-end gap-1">
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label="查看弹幕池"
              title="查看弹幕"
              onClick={() => setSelectedPool(pool)}
            >
              <EyeIcon />
            </Button>
            <Button
              type="button"
              size="icon-sm"
              variant="outline"
              aria-label="立即同步弹幕池"
              title="立即同步"
              disabled={sync.isPending}
              onClick={() => sync.mutate(pool.id)}
            >
              {sync.isPending ? <Spinner /> : <RefreshCwIcon />}
            </Button>
          </div>
        ),
      },
    ],
    [sync],
  );

  function search(event: FormEvent) {
    event.preventDefault();
    setPage(1);
    setQuery(draftQuery.trim());
  }

  function submitPool(event: FormEvent) {
    event.preventDefault();
    const body: CreatePoolInput = { p: Number(part) };
    if (identifierType === "bvid") {
      body.bvid = identifier.trim();
    } else if (identifierType === "aid") {
      body.aid = Number(identifier);
    } else {
      body.cid = Number(identifier);
    }
    create.mutate(
      body,
      {
        onSuccess: () => {
          setCreateOpen(false);
          setIdentifierType("bvid");
          setIdentifier("");
          setPart("1");
        },
      },
    );
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>弹幕池</CardTitle>
          <CardDescription>
            上次获取后的 10 分钟内只返回缓存；超过后由下一次请求触发增量更新，也可手动立即更新。
          </CardDescription>
          <CardAction>
            <Button type="button" onClick={() => setCreateOpen(true)}>
              <PlusIcon data-icon="inline-start" />
              添加弹幕池
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          <form onSubmit={search}>
            <FieldGroup>
              <Field orientation="horizontal">
                <Input
                  aria-label="搜索 BVID 或 CID"
                  value={draftQuery}
                  placeholder="搜索 BVID 或 CID"
                  onChange={(event) => setDraftQuery(event.target.value)}
                />
                <Button type="submit" variant="outline">
                  <SearchIcon data-icon="inline-start" />
                  查询
                </Button>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
        <CardContent className="p-0">
          {pools.isError ? (
            <div className="p-6">
              <QueryError
                error={pools.error}
                retry={() => void pools.refetch()}
              />
            </div>
          ) : pools.isPending ? (
            <LoadingTable />
          ) : (
            <DataTable
              rows={pools.data?.list ?? []}
              columns={columns}
              rowKey={(pool) => String(pool.id)}
              emptyTitle="暂无 Bilibili 弹幕池"
              emptyDescription="输入 BVID、AID 或 CID 与分 P 后开始缓存弹幕。"
            />
          )}
          {pools.data ? (
            <ListPagination
              meta={{ page, pageSize: 20, total: pools.data.total }}
              onPageChange={setPage}
            />
          ) : null}
        </CardContent>
      </Card>
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <form className="flex flex-col gap-5" onSubmit={submitPool}>
            <DialogHeader>
              <DialogTitle>添加 Bilibili 弹幕池</DialogTitle>
              <DialogDescription>
                输入任一视频标识，系统会创建弹幕池并立即从上游执行第一次增量同步。
              </DialogDescription>
            </DialogHeader>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="pool-identifier-type">标识类型</FieldLabel>
                <Select
                  value={identifierType}
                  onValueChange={(value) =>
                    setIdentifierType(
                      (value as PoolIdentifierType | null) ?? "bvid",
                    )
                  }
                >
                  <SelectTrigger id="pool-identifier-type" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="bvid">BVID</SelectItem>
                      <SelectItem value="aid">AID</SelectItem>
                      <SelectItem value="cid">CID</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="pool-identifier">
                  {identifierType.toUpperCase()}
                </FieldLabel>
                <Input
                  id="pool-identifier"
                  type={identifierType === "bvid" ? "text" : "number"}
                  min={identifierType === "bvid" ? undefined : "1"}
                  step={identifierType === "bvid" ? undefined : "1"}
                  value={identifier}
                  placeholder={
                    identifierType === "bvid"
                      ? "例如 BV1xx411c7mD"
                      : `请输入 ${identifierType.toUpperCase()}`
                  }
                  required
                  onChange={(event) => setIdentifier(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="pool-part">分 P</FieldLabel>
                <Input
                  id="pool-part"
                  type="number"
                  min="1"
                  step="1"
                  value={part}
                  required
                  onChange={(event) => setPart(event.target.value)}
                />
              </Field>
            </FieldGroup>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setCreateOpen(false)}
              >
                取消
              </Button>
              <Button type="submit" disabled={create.isPending}>
                {create.isPending ? <Spinner data-icon="inline-start" /> : null}
                创建并同步
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <PoolDanmakuDialog
        pool={selectedPool}
        onOpenChange={(open) => {
          if (!open) setSelectedPool(null);
        }}
      />
    </>
  );
}

function PoolDanmakuDialog({
  pool,
  onOpenChange,
}: {
  pool: BilibiliPool | null;
  onOpenChange: (open: boolean) => void;
}) {
  const [page, setPage] = useState(1);
  const [draftQuery, setDraftQuery] = useState("");
  const [query, setQuery] = useState("");
  const [blocked, setBlocked] = useState("all");

  useEffect(() => {
    setPage(1);
    setDraftQuery("");
    setQuery("");
    setBlocked("all");
  }, [pool?.id]);

  const danmaku = useQuery({
    queryKey: ["bilibili-pool-danmaku", pool?.id, page, query, blocked],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<BilibiliPoolDanmaku>>>(
          `/api/admin/bilibili/pools/${pool?.id}/danmaku`,
          {
            page,
            size: 20,
            query,
            blocked: blocked === "all" ? "" : blocked,
          },
        )
      ).data,
    enabled: Boolean(pool),
  });
  const block = useApiMutation<
    { id: number; blocked: boolean },
    ApiResponse<null>
  >({
    mutationFn: ({ id, blocked }) =>
      apiPatch(`/api/admin/bilibili/danmaku/${id}/blocked`, { blocked }),
    successMessage: "弹幕屏蔽状态已更新",
    invalidate: [["bilibili-pool-danmaku"], ["bilibili-pools"]],
  });
  const columns: DataColumn<BilibiliPoolDanmaku>[] = [
    {
      key: "content",
      label: "弹幕内容",
      render: (item) => (
        <div className="max-w-xl">
          <p className="truncate font-medium" title={item.data.text ?? ""}>
            {item.data.text || "—"}
          </p>
          <p className="text-xs text-muted-foreground">
            {item.data.time.toFixed(2)} 秒 · 时间戳 {item.data.timeStamp}
          </p>
        </div>
      ),
    },
    {
      key: "status",
      label: "状态",
      render: (item) => (
        <div className="flex flex-wrap gap-2">
          <Badge variant={item.isBlocked ? "destructive" : "secondary"}>
            {item.isBlocked ? "已屏蔽" : "正常"}
          </Badge>
          {item.isBlocked && !item.manuallyBlocked ? (
            <Badge variant="outline">关键词命中</Badge>
          ) : null}
        </div>
      ),
    },
    {
      key: "manual",
      label: "手动屏蔽",
      className: "w-28",
      render: (item) => (
        <Switch
          aria-label={`手动屏蔽弹幕 ${item.id}`}
          checked={item.manuallyBlocked}
          disabled={block.isPending}
          onCheckedChange={(value) =>
            block.mutate({ id: item.id, blocked: value })
          }
        />
      ),
    },
  ];

  function search(event: FormEvent) {
    event.preventDefault();
    setPage(1);
    setQuery(draftQuery.trim());
  }

  return (
    <Dialog open={Boolean(pool)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle>{pool ? poolLabel(pool) : "弹幕池详情"}</DialogTitle>
          <DialogDescription>
            关键词命中的弹幕会自动隐藏；手动屏蔽状态可在这里单独调整。
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={search}>
          <FieldGroup>
            <Field orientation="horizontal">
              <Input
                aria-label="搜索弹幕内容"
                value={draftQuery}
                placeholder="搜索弹幕内容"
                onChange={(event) => setDraftQuery(event.target.value)}
              />
              <Select
                items={[
                  { value: "all", label: "全部状态" },
                  { value: "false", label: "正常" },
                  { value: "true", label: "已屏蔽" },
                ]}
                value={blocked}
                onValueChange={(value) => {
                  setBlocked(value ?? "all");
                  setPage(1);
                }}
              >
                <SelectTrigger className="w-full sm:w-36">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="all">全部状态</SelectItem>
                    <SelectItem value="false">正常</SelectItem>
                    <SelectItem value="true">已屏蔽</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Button type="submit" variant="outline">
                <SearchIcon data-icon="inline-start" />
                查询
              </Button>
            </Field>
          </FieldGroup>
        </form>
        <div className="overflow-hidden rounded-xl border">
          {danmaku.isError ? (
            <div className="p-6">
              <QueryError
                error={danmaku.error}
                retry={() => void danmaku.refetch()}
              />
            </div>
          ) : danmaku.isPending ? (
            <LoadingTable />
          ) : (
            <DataTable
              rows={danmaku.data?.list ?? []}
              columns={columns}
              rowKey={(item) => String(item.id)}
              emptyTitle="没有匹配的弹幕"
              emptyDescription="调整内容或屏蔽状态后重新查询。"
            />
          )}
          {danmaku.data ? (
            <ListPagination
              meta={{ page, pageSize: 20, total: danmaku.data.total }}
              onPageChange={setPage}
            />
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function KeywordPanel({ pools }: { pools: BilibiliPool[] }) {
  const [scope, setScope] = useState("global");
  const [keyword, setKeyword] = useState("");
  const keywords = useQuery({
    queryKey: ["bilibili-keywords"],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<BilibiliKeyword[]>>(
          "/api/admin/bilibili/keywords",
        )
      ).data,
  });
  const create = useApiMutation<
    { poolId: number | null; keyword: string },
    ApiResponse<BilibiliKeyword>
  >({
    mutationFn: (body) => apiPost("/api/admin/bilibili/keywords", body),
    successMessage: "过滤关键词已添加",
    invalidate: [
      ["bilibili-keywords"],
      ["bilibili-pools"],
      ["bilibili-pool-danmaku"],
    ],
  });
  const remove = useApiMutation<number, ApiResponse<null>>({
    mutationFn: (id) => apiDelete(`/api/admin/bilibili/keywords/${id}`),
    successMessage: "过滤关键词已删除",
    invalidate: [
      ["bilibili-keywords"],
      ["bilibili-pools"],
      ["bilibili-pool-danmaku"],
    ],
  });
  const columns: DataColumn<BilibiliKeyword>[] = [
    {
      key: "keyword",
      label: "关键词",
      render: (item) => <span className="font-medium">{item.keyword}</span>,
    },
    {
      key: "scope",
      label: "作用范围",
      render: (item) =>
        item.poolId ? (
          <Badge variant="secondary">
            {keywordPoolLabel(item)}
          </Badge>
        ) : (
          <Badge>全局</Badge>
        ),
    },
    {
      key: "created",
      label: "创建时间",
      className: "whitespace-nowrap",
      render: (item) => formatDateTime(item.createTime),
    },
    {
      key: "actions",
      label: "操作",
      className: "w-16 text-right",
      render: (item) => (
        <ConfirmAction
          trigger={
            <Button
              type="button"
              size="icon-sm"
              variant="destructive"
              aria-label="删除过滤关键词"
              title="删除"
            >
              <Trash2Icon />
            </Button>
          }
          title="删除这个过滤关键词？"
          description="删除后，只有因该关键词而隐藏的弹幕会恢复为可见。"
          destructive
          pending={remove.isPending}
          onConfirm={() => remove.mutate(item.id)}
        />
      ),
    },
  ];

  function submit(event: FormEvent) {
    event.preventDefault();
    create.mutate(
      {
        poolId: scope === "global" ? null : Number(scope),
        keyword: keyword.trim(),
      },
      { onSuccess: () => setKeyword("") },
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>过滤关键词</CardTitle>
        <CardDescription>
          全局规则作用于所有弹幕池；池级规则只作用于指定 CID 弹幕池。弹幕数据仍会完整保留。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <FieldGroup className="grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
            <Field>
              <FieldLabel htmlFor="keyword-scope">作用范围</FieldLabel>
              <Select
                items={[
                  { value: "global", label: "全部弹幕池" },
                  ...pools.map((pool) => ({
                    value: String(pool.id),
                    label: poolLabel(pool),
                  })),
                ]}
                value={scope}
                onValueChange={(value) => setScope(value ?? "global")}
              >
                <SelectTrigger id="keyword-scope" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="global">全部弹幕池</SelectItem>
                    {pools.map((pool) => (
                      <SelectItem key={pool.id} value={String(pool.id)}>
                        {poolLabel(pool)}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field>
              <FieldLabel htmlFor="keyword-value">关键词</FieldLabel>
              <div className="flex gap-2">
                <Input
                  id="keyword-value"
                  value={keyword}
                  maxLength={200}
                  placeholder="输入需要过滤的内容片段"
                  required
                  onChange={(event) => setKeyword(event.target.value)}
                />
                <Button type="submit" disabled={create.isPending}>
                  {create.isPending ? (
                    <Spinner data-icon="inline-start" />
                  ) : (
                    <ShieldBanIcon data-icon="inline-start" />
                  )}
                  添加
                </Button>
              </div>
              <FieldDescription>
                匹配不区分大小写，命中后默认从公开返回中隐藏。
              </FieldDescription>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
      <CardContent className="p-0">
        {keywords.isError ? (
          <div className="p-6">
            <QueryError
              error={keywords.error}
              retry={() => void keywords.refetch()}
            />
          </div>
        ) : keywords.isPending ? (
          <LoadingTable />
        ) : (
          <DataTable
            rows={keywords.data ?? []}
            columns={columns}
            rowKey={(item) => String(item.id)}
            emptyTitle="暂无过滤关键词"
            emptyDescription="添加全局或弹幕池级关键词后，命中的弹幕会自动隐藏。"
          />
        )}
      </CardContent>
    </Card>
  );
}

function BindingPanel({ pools }: { pools: BilibiliPool[] }) {
  const [page, setPage] = useState(1);
  const [draftQuery, setDraftQuery] = useState("");
  const [query, setQuery] = useState("");
  const [vid, setVid] = useState("");
  const [poolID, setPoolID] = useState("");
  const [offset, setOffset] = useState("0");
  const bindings = useQuery({
    queryKey: ["bilibili-bindings", page, query],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<BilibiliBinding>>>(
          "/api/admin/bilibili/bindings",
          { page, size: 20, query },
        )
      ).data,
  });
  const save = useApiMutation<
    { vid: string; poolId: number; offset: number },
    ApiResponse<BilibiliBinding>
  >({
    mutationFn: (body) => apiPost("/api/admin/bilibili/bindings", body),
    successMessage: "视频关联已保存",
    invalidate: [["bilibili-bindings"], ["bilibili-pools"]],
  });
  const remove = useApiMutation<number, ApiResponse<null>>({
    mutationFn: (id) => apiDelete(`/api/admin/bilibili/bindings/${id}`),
    successMessage: "视频关联已删除",
    invalidate: [["bilibili-bindings"], ["bilibili-pools"]],
  });
  const columns: DataColumn<BilibiliBinding>[] = [
    {
      key: "vid",
      label: "本地视频 ID",
      render: (item) => (
        <span className="font-mono text-xs">{item.vid}</span>
      ),
    },
    {
      key: "pool",
      label: "Bilibili 弹幕池",
      render: (item) => (
        <div>
          <p>{poolLabel(item)}</p>
          <p className="font-mono text-xs text-muted-foreground">
            CID {item.cid}
          </p>
        </div>
      ),
    },
    {
      key: "offset",
      label: "时间偏移",
      render: (item) => (
        <Badge variant="outline">{offsetLabel(item.offset)}</Badge>
      ),
    },
    {
      key: "updated",
      label: "最后修改",
      className: "whitespace-nowrap",
      render: (item) => formatDateTime(item.updateTime),
    },
    {
      key: "actions",
      label: "操作",
      className: "w-20 whitespace-nowrap text-right",
      render: (item) => (
        <div className="flex justify-end gap-1">
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label="编辑关联"
            title="编辑"
            onClick={() => {
              setVid(item.vid);
              setPoolID(String(item.poolId));
              setOffset(String(item.offset));
            }}
          >
            <PencilIcon />
          </Button>
          <ConfirmAction
            trigger={
              <Button
                type="button"
                size="icon-sm"
                variant="destructive"
                aria-label="删除关联"
                title="删除"
              >
                <Trash2Icon />
              </Button>
            }
            title="删除这个视频关联？"
            description="之后请求该视频 ID 时将不再合并这个 Bilibili 弹幕池。"
            destructive
            pending={remove.isPending}
            onConfirm={() => remove.mutate(item.id)}
          />
        </div>
      ),
    },
  ];

  function submit(event: FormEvent) {
    event.preventDefault();
    save.mutate(
      { vid: vid.trim(), poolId: Number(poolID), offset: Number(offset) },
      {
        onSuccess: () => {
          setVid("");
          setPoolID("");
          setOffset("0");
        },
      },
    );
  }

  function search(event: FormEvent) {
    event.preventDefault();
    setPage(1);
    setQuery(draftQuery.trim());
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>视频关联</CardTitle>
        <CardDescription>
          将以 CID 唯一标识的弹幕池合并到本地视频 ID；偏移量只作用于合并进来的 Bilibili 弹幕。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <FieldGroup className="grid gap-4 md:grid-cols-3">
            <Field>
              <FieldLabel htmlFor="binding-vid">本地视频 ID</FieldLabel>
              <Input
                id="binding-vid"
                value={vid}
                maxLength={36}
                required
                onChange={(event) => setVid(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="binding-pool">Bilibili 弹幕池</FieldLabel>
              <Select
                items={pools.map((pool) => ({
                  value: String(pool.id),
                  label: poolLabel(pool),
                }))}
                value={poolID}
                onValueChange={(value) => setPoolID(value ?? "")}
              >
                <SelectTrigger id="binding-pool" className="w-full">
                  <SelectValue placeholder="选择弹幕池" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {pools.map((pool) => (
                      <SelectItem key={pool.id} value={String(pool.id)}>
                        {poolLabel(pool)}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field>
              <FieldLabel htmlFor="binding-offset">偏移量（秒）</FieldLabel>
              <div className="flex gap-2">
                <Input
                  id="binding-offset"
                  type="number"
                  step="any"
                  value={offset}
                  required
                  onChange={(event) => setOffset(event.target.value)}
                />
                <Button type="submit" disabled={save.isPending || !poolID}>
                  {save.isPending ? (
                    <Spinner data-icon="inline-start" />
                  ) : (
                    <LinkIcon data-icon="inline-start" />
                  )}
                  保存
                </Button>
              </div>
              <FieldDescription>正数延后，负数提前。</FieldDescription>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
      <CardContent>
        <form onSubmit={search}>
          <FieldGroup>
            <Field orientation="horizontal">
              <Input
                aria-label="搜索视频关联"
                value={draftQuery}
                placeholder="搜索本地视频 ID 或 BVID"
                onChange={(event) => setDraftQuery(event.target.value)}
              />
              <Button type="submit" variant="outline">
                <SearchIcon data-icon="inline-start" />
                查询
              </Button>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
      <CardContent className="p-0">
        {bindings.isError ? (
          <div className="p-6">
            <QueryError
              error={bindings.error}
              retry={() => void bindings.refetch()}
            />
          </div>
        ) : bindings.isPending ? (
          <LoadingTable />
        ) : (
          <DataTable
            rows={bindings.data?.list ?? []}
            columns={columns}
            rowKey={(item) => String(item.id)}
            emptyTitle="暂无视频关联"
            emptyDescription="选择弹幕池并关联到一个本地视频 ID。"
          />
        )}
        {bindings.data ? (
          <ListPagination
            meta={{ page, pageSize: 20, total: bindings.data.total }}
            onPageChange={setPage}
          />
        ) : null}
      </CardContent>
    </Card>
  );
}
