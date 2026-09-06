import { createContext, useContext, useMemo, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  EyeIcon,
  LinkIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  Trash2Icon,
} from "lucide-react";

import { apiDelete, apiGet, apiPatch, apiPost } from "@/api/client";
import type {
  ApiResponse,
  CatalogBinding,
  CatalogKeyword,
  CatalogPool,
  CatalogPoolDanmaku,
  ManagedVideo,
} from "@/api/types";
import { ConfirmAction } from "@/components/confirm-action";
import { ThirdPartySearch } from "@/components/third-party-search";
import { DataTable, type DataColumn } from "@/components/data-table";
import { ListPagination } from "@/components/list-pagination";
import { LoadingTable } from "@/components/loading-table";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { SearchableSelect } from "@/components/searchable-select";
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
import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useApiMutation } from "@/hooks/use-api-mutation";
import { catalogPoolLabel, catalogLabels, catalogIDLabels, type CatalogSource } from "@/lib/catalog";
const CatalogContext = createContext<CatalogSource>("bahamut");
function useCatalog() { const source = useContext(CatalogContext); return { source, label: catalogLabels[source], idLabel: catalogIDLabels[source] }; }
export function CatalogPage({source}: {source: CatalogSource}) { return <CatalogContext value={source}><CatalogContent key={source} /></CatalogContext>; }
import { Switch } from "@/components/ui/switch";
import { formatDateTime } from "@/lib/format";

type Paged<T> = { total: number; list: T[] };
type PoolMutationResult = { pool: CatalogPool; inserted: number };

function CatalogContent() {
  const {source, label} = useCatalog();
  const poolOptions = useQuery({
    queryKey: [`${source}-pool-options`],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<CatalogPool>>>(`/api/admin/${source}/pools`, {
          page: 1,
          size: 500,
        })
      ).data.list,
  });

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="第三方弹幕库"
        title={label}
        description={`持久化管理 ${label} 弹幕池、视频关联和过滤规则`}
      />
      <Tabs defaultValue="pools">
        <TabsList>
          <TabsTrigger value="pools">弹幕池</TabsTrigger>
          <TabsTrigger value="keywords">关键词过滤</TabsTrigger>
        </TabsList>
        <TabsContent value="pools" className="flex flex-col gap-4">
          <PoolPanel />
        </TabsContent>
        <TabsContent value="keywords" className="flex flex-col gap-4">
          <KeywordPanel pools={poolOptions.data ?? []} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function PoolPanel() {
  const {source, label, idLabel} = useCatalog();
  const [page, setPage] = useState(1);
  const [draftQuery, setDraftQuery] = useState("");
  const [query, setQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [episodeId, setEpisodeID] = useState("");
  const [selectedPool, setSelectedPool] = useState<CatalogPool | null>(null);
  const [bindingPool, setBindingPool] = useState<CatalogPool | null>(null);
  const pools = useQuery({
    queryKey: [`${source}-pools`, page, query],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<CatalogPool>>>(`/api/admin/${source}/pools`, {
          page,
          size: 20,
          query,
        })
      ).data,
  });
  const create = useApiMutation<
    { episodeId: string },
    ApiResponse<PoolMutationResult>
  >({
    mutationFn: (body) => apiPost(`/api/admin/${source}/pools`, body),
    successMessage: "弹幕池已创建并同步",
    invalidate: [["videos"], ["video"], ["video-heatmap"], ["catalog-pool-options"], [`${source}-pools`], [`${source}-pool-options`]],
  });
  const sync = useApiMutation<number, ApiResponse<PoolMutationResult>>({
    mutationFn: (id) => apiPost(`/api/admin/${source}/pools/${id}/sync`),
    successMessage: "弹幕池已增量同步",
    invalidate: [
      ["videos"], ["video"], ["video-heatmap"],
      [`${source}-pools`],
      [`${source}-pool-options`],
      [`${source}-pool-danmaku`],
    ],
  });
  const columns = useMemo<DataColumn<CatalogPool>[]>(
    () => [
      {
        key: "pool",
        label: "弹幕池",
        render: (pool) => <Button type="button" variant="link" className="h-auto max-w-full justify-start p-0 text-left" onClick={() => setSelectedPool(pool)}><span className="truncate font-mono">{catalogPoolLabel(pool)}</span></Button>,
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
        className: "w-32 whitespace-nowrap text-right",
        render: (pool) => (
          <div className="flex justify-end gap-1">
            <Button
              type="button"
              size="icon-sm"
              variant="secondary"
              aria-label="查看 第三方弹幕池"
              title="查看弹幕"
              onClick={() => setSelectedPool(pool)}
            >
              <EyeIcon />
            </Button>
            <Button
              type="button"
              size="icon-sm"
              variant="secondary"
              aria-label="关联视频"
              title="关联视频"
              onClick={() => setBindingPool(pool)}
            >
              <LinkIcon />
            </Button>
            <Button
              type="button"
              size="icon-sm"
              variant="secondary"
              aria-label="立即同步 第三方弹幕池"
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

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>弹幕池</CardTitle>
          <CardDescription>
            同步窗口内直接返回缓存；超过窗口后由下一次请求被动增量更新，也可手动立即更新
          </CardDescription>
          <CardAction>
            <Button type="button" onClick={() => setCreateOpen(true)}>
              <PlusIcon data-icon="inline-start" />
              添加弹幕池
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              setPage(1);
              setQuery(draftQuery.trim());
            }}
          >
            <FieldGroup>
              <Field orientation="horizontal">
                <Input
                  aria-label="搜索 视频 ID"
                  value={draftQuery}
                  placeholder="搜索 视频 ID"
                  onChange={(event) => setDraftQuery(event.target.value)}
                />
                <Button type="submit" variant="outline">
                  <SearchIcon data-icon="inline-start" />查询
                </Button>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
        <CardContent className="p-0">
          {pools.isError ? (
            <div className="p-6">
              <QueryError error={pools.error} retry={() => void pools.refetch()} />
            </div>
          ) : pools.isPending ? (
            <LoadingTable />
          ) : (
            <DataTable
              rows={pools.data?.list ?? []}
              columns={columns}
              rowKey={(pool) => String(pool.id)}
              emptyTitle="暂无 第三方弹幕池"
              emptyDescription="搜索作品并选择剧集，或输入剧集 ID 创建弹幕池"
            />
          )}
        </CardContent>
        {pools.data ? (
          <ListPagination
            meta={{ page, pageSize: 20, total: pools.data.total }}
            onPageChange={setPage}
          />
        ) : null}
      </Card>
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-xl">
          <form
            className="flex flex-col gap-5"
            onSubmit={(event) => {
              event.preventDefault();
              create.mutate(
                { episodeId: episodeId.trim() },
                {
                  onSuccess: () => {
                    setCreateOpen(false);
                    setEpisodeID("");
                  },
                },
              );
            }}
          >
            <DialogHeader>
              <DialogTitle>添加{label}弹幕池</DialogTitle>
              <DialogDescription>
                搜索作品选择剧集，或手动输入视频标识及链接；创建时立即获取弹幕并建立本地缓存
              </DialogDescription>
            </DialogHeader>
            {createOpen ? <ThirdPartySearch source={source} episodeId={episodeId} onSelect={setEpisodeID} disabled={create.isPending} /> : null}
            <FieldGroup>
            <Field data-disabled={create.isPending}>
              <FieldLabel htmlFor="catalog-pool-episodeId">{idLabel}</FieldLabel>
              <Input
                id="catalog-pool-episodeId"
                disabled={create.isPending}
                value={episodeId}
                maxLength={2048}
                inputMode="text"
                required
                onChange={(event) => setEpisodeID(event.target.value)}
              />
            </Field>
            </FieldGroup>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                取消
              </Button>
              <Button type="submit" disabled={create.isPending || !episodeId.trim()}>
                {create.isPending ? <Spinner data-icon="inline-start" /> : null}
                创建并同步
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <PoolDanmakuDialog pool={selectedPool} onOpenChange={(open) => !open && setSelectedPool(null)} />
      <BindingDialog pool={bindingPool} onOpenChange={(open) => !open && setBindingPool(null)} />
    </>
  );
}

function BindingDialog({ pool, onOpenChange }: { pool: CatalogPool | null; onOpenChange: (open: boolean) => void }) {
  const {source} = useCatalog();
  const [videoID, setVideoID] = useState("");
  const [offset, setOffset] = useState("0");
  const videos = useQuery({
    queryKey: [`${source}-binding-video-options`],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<ManagedVideo>>>("/api/admin/videos", {
          page: 1,
          size: 500,
          deleted: false,
        })
      ).data.list,
    enabled: Boolean(pool),
  });
  const bind = useApiMutation<
    { videoID: number; poolId: number; offset: number },
    ApiResponse<CatalogBinding>
  >({
    mutationFn: ({ videoID: targetVideoID, ...body }) =>
      apiPost(`/api/admin/videos/${targetVideoID}/${source}-bindings`, body),
    successMessage: "弹幕池已关联到视频",
    invalidate: [["videos"], ["video"], ["video-heatmap"], ["videos"], ["video"], [`${source}-pools`]],
  });

  function submit(event: FormEvent) {
    event.preventDefault();
    if (!pool) return;
    bind.mutate(
      { videoID: Number(videoID), poolId: pool.id, offset: Number(offset) },
      { onSuccess: () => onOpenChange(false) },
    );
  }

  return (
    <Dialog open={Boolean(pool)} onOpenChange={onOpenChange}>
      <DialogContent>
        <form className="flex flex-col gap-5" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>关联视频</DialogTitle>
            <DialogDescription>
              将 {pool ? catalogPoolLabel(pool) : "当前弹幕池"} 关联到现有视频；重复关联会更新偏移量
            </DialogDescription>
          </DialogHeader>
          {videos.isError ? (
            <QueryError error={videos.error} retry={() => void videos.refetch()} />
          ) : (
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="catalog-binding-video">视频</FieldLabel>
                <SearchableSelect
                  id="catalog-binding-video"
                  options={(videos.data ?? []).map((video) => ({
                    value: String(video.id),
                    label: video.name ? `${video.name} (${video.vid})` : video.vid,
                  }))}
                  value={videoID}
                  disabled={videos.isPending}
                  placeholder={videos.isPending ? "正在加载视频" : "选择视频"}
                  searchPlaceholder="搜索视频"
                  onValueChange={setVideoID}
                />
                {!videos.isPending && videos.data?.length === 0 ? (
                  <FieldDescription>暂无可关联的视频，请先在视频管理中添加</FieldDescription>
                ) : null}
              </Field>
              <Field>
                <FieldLabel htmlFor="catalog-binding-offset">偏移量（秒）</FieldLabel>
                <Input id="catalog-binding-offset" type="number" step="any" value={offset} required onChange={(event) => setOffset(event.target.value)} />
              </Field>
            </FieldGroup>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
            <Button type="submit" disabled={bind.isPending || !videoID || videos.isPending}>
              {bind.isPending ? <Spinner data-icon="inline-start" /> : <LinkIcon data-icon="inline-start" />}
              关联
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function PoolDanmakuDialog({ pool, onOpenChange }: { pool: CatalogPool | null; onOpenChange: (open: boolean) => void }) {
  const {source} = useCatalog();
  const [page, setPage] = useState(1);
  const [draftQuery, setDraftQuery] = useState("");
  const [query, setQuery] = useState("");
  const [blocked, setBlocked] = useState("all");
  const danmaku = useQuery({
    queryKey: [`${source}-pool-danmaku`, pool?.id, page, query, blocked],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<CatalogPoolDanmaku>>>(
          `/api/admin/${source}/pools/${pool?.id}/danmaku`,
          { page, size: 20, query, blocked: blocked === "all" ? "" : blocked },
        )
      ).data,
    enabled: Boolean(pool),
  });
  const blockMutation = useApiMutation<
    { id: number; blocked: boolean },
    ApiResponse<null>
  >({
    mutationFn: ({ id, blocked: value }) =>
      apiPatch(`/api/admin/${source}/danmaku/${id}/blocked`, { blocked: value }),
    successMessage: "屏蔽状态已更新",
    invalidate: [["videos"], ["video"], ["video-heatmap"], [`${source}-pool-danmaku`], [`${source}-pools`]],
  });
  const columns: DataColumn<CatalogPoolDanmaku>[] = [
    {
      key: "content",
      label: "弹幕内容",
      render: (item) => <p className="max-w-xl break-words">{item.data.text || "—"}</p>,
    },
    {
      key: "time",
      label: "出现时间",
      className: "whitespace-nowrap",
      render: (item) => `${item.data.time.toFixed(2)} 秒`,
    },
    {
      key: "status",
      label: "状态",
      render: (item) => (
        <Badge variant={item.isBlocked ? "destructive" : "secondary"}>
          {item.isBlocked ? "已屏蔽" : "正常"}
        </Badge>
      ),
    },
    {
      key: "blocked",
      label: "手动屏蔽",
      render: (item) => (
        <Switch
          aria-label={`${item.manuallyBlocked ? "取消屏蔽" : "屏蔽"}弹幕`}
          checked={item.manuallyBlocked}
          disabled={blockMutation.isPending}
          onCheckedChange={(value) => blockMutation.mutate({ id: item.id, blocked: value })}
        />
      ),
    },
  ];

  return (
    <Dialog open={Boolean(pool)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle>{pool ? catalogPoolLabel(pool) : "弹幕池详情"}</DialogTitle>
          <DialogDescription>关键词命中的弹幕会自动隐藏；手动屏蔽状态可在这里单独调整</DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            setPage(1);
            setQuery(draftQuery.trim());
          }}
        >
          <FieldGroup>
            <Field orientation="horizontal">
              <Input aria-label="搜索弹幕内容" value={draftQuery} placeholder="搜索弹幕内容" onChange={(event) => setDraftQuery(event.target.value)} />
              <SearchableSelect
                options={[
                  { value: "all", label: "全部状态" },
                  { value: "false", label: "正常" },
                  { value: "true", label: "已屏蔽" },
                ]}
                value={blocked}
                className="sm:w-36"
                searchPlaceholder="搜索状态"
                onValueChange={(value) => {
                  setBlocked(value);
                  setPage(1);
                }}
              />
              <Button type="submit" variant="outline"><SearchIcon data-icon="inline-start" />查询</Button>
            </Field>
          </FieldGroup>
        </form>
        <div className="overflow-hidden rounded-xl border">
          {danmaku.isError ? (
            <div className="p-6"><QueryError error={danmaku.error} retry={() => void danmaku.refetch()} /></div>
          ) : danmaku.isPending ? (
            <LoadingTable />
          ) : (
            <DataTable rows={danmaku.data?.list ?? []} columns={columns} rowKey={(item) => String(item.id)} emptyTitle="没有匹配的弹幕" emptyDescription="请调整筛选条件后重试" />
          )}
          {danmaku.data ? <ListPagination meta={{ page, pageSize: 20, total: danmaku.data.total }} onPageChange={setPage} variant="inline" /> : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function KeywordPanel({ pools }: { pools: CatalogPool[] }) {
  const {source, label} = useCatalog();
  const [scope, setScope] = useState("global");
  const [keyword, setKeyword] = useState("");
  const keywords = useQuery({
    queryKey: [`${source}-keywords`],
    queryFn: async () => (await apiGet<ApiResponse<CatalogKeyword[]>>(`/api/admin/${source}/keywords`)).data,
  });
  const create = useApiMutation<
    { poolId: number | null; keyword: string },
    ApiResponse<CatalogKeyword>
  >({
    mutationFn: (body) => apiPost(`/api/admin/${source}/keywords`, body),
    successMessage: "关键词过滤规则已添加",
    invalidate: [["videos"], ["video"], ["video-heatmap"], [`${source}-keywords`], [`${source}-pools`], [`${source}-pool-danmaku`]],
  });
  const remove = useApiMutation<number, ApiResponse<null>>({
    mutationFn: (id) => apiDelete(`/api/admin/${source}/keywords/${id}`),
    successMessage: "关键词过滤规则已删除",
    invalidate: [["videos"], ["video"], ["video-heatmap"], [`${source}-keywords`], [`${source}-pools`], [`${source}-pool-danmaku`]],
  });
  const columns: DataColumn<CatalogKeyword>[] = [
    { key: "keyword", label: "关键词", render: (item) => <span className="font-medium">{item.keyword}</span> },
    { key: "scope", label: "作用范围", render: (item) => <Badge variant={item.poolId ? "secondary" : "default"}>{item.poolId ? catalogPoolLabel({episodeId: item.poolEpisodeId}) : "全局"}</Badge> },
    { key: "created", label: "创建时间", className: "whitespace-nowrap", render: (item) => formatDateTime(item.createTime) },
    {
      key: "actions",
      label: "操作",
      className: "w-16 text-right",
      render: (item) => (
        <ConfirmAction
          trigger={<Button type="button" size="icon-sm" variant="destructive" aria-label="删除关键词过滤规则"><Trash2Icon /></Button>}
          title="删除这条关键词过滤规则？"
          description="删除后，匹配的弹幕将不再因这条规则被自动屏蔽"
          destructive
          pending={remove.isPending}
          onConfirm={() => remove.mutate(item.id)}
        />
      ),
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>关键词过滤</CardTitle>
        <CardDescription>全局规则作用于所有{label}弹幕池；池级规则只作用于指定视频 ID；弹幕数据仍会完整保留</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            create.mutate(
              { poolId: scope === "global" ? null : Number(scope), keyword: keyword.trim() },
              { onSuccess: () => setKeyword("") },
            );
          }}
        >
          <FieldGroup className="grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
            <Field>
              <FieldLabel htmlFor="catalog-keyword-scope">作用范围</FieldLabel>
              <SearchableSelect
                id="catalog-keyword-scope"
                options={[
                  { value: "global", label: "全部弹幕池" },
                  ...pools.map((pool) => ({ value: String(pool.id), label: catalogPoolLabel(pool) })),
                ]}
                value={scope}
                searchPlaceholder="搜索弹幕池"
                onValueChange={setScope}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="catalog-keyword-value">关键词</FieldLabel>
              <div className="flex gap-2">
                <Input id="catalog-keyword-value" value={keyword} maxLength={200} required onChange={(event) => setKeyword(event.target.value)} />
                <Button type="submit" disabled={create.isPending || !keyword.trim()}>
                  {create.isPending ? <Spinner data-icon="inline-start" /> : <PlusIcon data-icon="inline-start" />}
                  添加
                </Button>
              </div>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
      <CardContent className="p-0">
        {keywords.isError ? (
          <div className="p-6"><QueryError error={keywords.error} retry={() => void keywords.refetch()} /></div>
        ) : keywords.isPending ? (
          <LoadingTable />
        ) : (
          <DataTable rows={keywords.data ?? []} columns={columns} rowKey={(item) => String(item.id)} emptyTitle="暂无关键词过滤规则" emptyDescription="添加全局或弹幕池级关键词规则" />
        )}
      </CardContent>
    </Card>
  );
}
