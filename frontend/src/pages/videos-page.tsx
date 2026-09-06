import { AddDanmakuForm } from "@/components/add-danmaku-form";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  DownloadIcon,
  LinkIcon,
  PencilIcon,
  PlusIcon,
  RefreshCcwIcon,
  SearchIcon,
  Trash2Icon,
} from "lucide-react";
import { Area, AreaChart, CartesianGrid, XAxis } from "recharts";
import { useNavigate } from "react-router-dom";

import { apiDelete, apiGet, apiPatch, apiPost, apiPut } from "@/api/client";
import type {
  ApiResponse,
  BilibiliBinding,
  BilibiliPool,
  ExternalBinding,
  ExternalPool,
  HeatmapPoint,
  IqiyiBinding,
  DandanplayBinding,
  DandanplayPool,
  IqiyiPool,
  ManagedVideo,
} from "@/api/types";
import { ConfirmAction } from "@/components/confirm-action";
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
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
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
import { useApiMutation } from "@/hooks/use-api-mutation";
import { dandanplayPoolLabel } from "@/lib/dandanplay";
import { formatDateTime } from "@/lib/format";

type Paged<T> = { total: number; list: T[] };

const exportFormats = [
  { value: "common.json", label: "本系统 JSON" },
  { value: "danuni.json", label: "DanUni JSON" },
  { value: "danuni.pb", label: "DanUni Protobuf" },
  { value: "bilibili.xml", label: "bilibili XML" },
  { value: "dplayer.json", label: "DPlayer JSON" },
  { value: "artplayer.json", label: "ArtPlayer JSON" },
  { value: "ddplay.json", label: "弹弹Play JSON" },
  { value: "vod.json", label: "VOD JSON" },
  { value: "baha.json", label: "巴哈姆特 JSON" },
  { value: "ass", label: "ASS 字幕" },
];

function poolLabel(pool: Pick<BilibiliPool, "bvid" | "aid" | "cid" | "p">): string {
  return pool.bvid
    ? `${pool.bvid} / AID ${pool.aid} / P${pool.p}`
    : `CID ${pool.cid} / P${pool.p}`;
}

function offsetLabel(offset: number): string {
  return `${offset > 0 ? "+" : ""}${offset} 秒`;
}

function formatDuration(milliseconds: number): string {
  const seconds = Math.max(0, Math.floor(milliseconds / 1000));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = seconds % 60;
  return hours
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(rest).padStart(2, "0")}`
    : `${minutes}:${String(rest).padStart(2, "0")}`;
}

export function VideosPage() {
  const [page, setPage] = useState(1);
  const [draftQuery, setDraftQuery] = useState("");
  const [query, setQuery] = useState("");
  const [deleted, setDeleted] = useState("false");
  const [createOpen, setCreateOpen] = useState(false);
  const [createVid, setCreateVid] = useState("");
  const [createName, setCreateName] = useState("");
  const [selected, setSelected] = useState<ManagedVideo | null>(null);
  const videos = useQuery({
    queryKey: ["videos", page, query, deleted],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<ManagedVideo>>>("/api/admin/videos", {
          page,
          size: 20,
          query,
          deleted,
        })
      ).data,
  });
  const bilibiliPools = useQuery({
    queryKey: ["bilibili-pool-options"],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<BilibiliPool>>>(
          "/api/admin/bilibili/pools",
          { page: 1, size: 500 },
        )
      ).data.list,
  });
  const iqiyiPools = useQuery({
    queryKey: ["iqiyi-pool-options"],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<IqiyiPool>>>("/api/admin/iqiyi/pools", {
          page: 1,
          size: 500,
        })
      ).data.list,
  });
  const dandanplayPools = useQuery({
    queryKey: ["dandanplay-pool-options"],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<DandanplayPool>>>("/api/admin/dandanplay/pools", {
          page: 1,
          size: 500,
        })
      ).data.list,
  });
  const externalPools = useQuery({
    queryKey: ["external-pool-options"],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<ExternalPool>>>("/api/admin/external", {
          page: 1,
          size: 500,
        })
      ).data.list,
  });
  const create = useApiMutation<
    { vid: string; name: string },
    ApiResponse<ManagedVideo>
  >({
    mutationFn: (body) => apiPost("/api/admin/videos", body),
    successMessage: "视频已创建",
    invalidate: [["videos"]],
  });
  const status = useApiMutation<
    { id: number; deleted: boolean },
    ApiResponse<null>
  >({
    mutationFn: ({ id, deleted }) =>
      apiPatch(`/api/admin/videos/${id}/status`, { deleted }),
    successMessage: "视频状态已更新",
    invalidate: [["videos"], ["video"]],
  });
  const columns = useMemo<DataColumn<ManagedVideo>[]>(
    () => [
      {
        key: "name",
        label: "视频",
        render: (video) => (
          <div className="flex max-w-80 flex-col gap-1">
            <Button
              type="button"
              variant="link"
              className="h-auto max-w-80 justify-start p-0 text-left"
              onClick={() => setSelected(video)}
            >
              <span className="truncate">{video.name || "未命名视频"}</span>
            </Button>
            <span className="truncate font-mono text-xs text-muted-foreground" title={video.vid}>{video.vid}</span>
          </div>
        ),
      },
      {
        key: "total",
        label: "总弹幕",
        render: (video) => (
          <span className="tabular-nums">{video.danmakuCount + video.thirdPartyDanmakuCount} 条</span>
        ),
      },
      {
        key: "pools",
        label: "弹幕池",
        render: (video) => (
          <div className="flex flex-wrap gap-2">
            <Badge variant="secondary">系统弹幕池 {video.danmakuCount} 条</Badge>
            <Badge variant="outline">第三方弹幕池 {video.bilibiliPoolCount + video.iqiyiPoolCount + video.dandanplayPoolCount + video.externalPoolCount} 个 {video.thirdPartyDanmakuCount} 条</Badge>
          </div>
        ),
      },
      {
        key: "status",
        label: "状态",
        render: (video) => (
          <Badge variant={video.isDelete ? "destructive" : "secondary"}>
            {video.isDelete ? "已删除" : "正常"}
          </Badge>
        ),
      },
      {
        key: "updated",
        label: "最后修改",
        className: "whitespace-nowrap",
        render: (video) => formatDateTime(video.updateTime),
      },
      {
        key: "actions",
        label: "操作",
        className: "w-24 whitespace-nowrap text-right",
        render: (video) => (
          <div className="flex justify-end gap-1">
            <Button
              type="button"
              size="icon-sm"
              variant="secondary"
              aria-label="编辑视频"
              title="编辑"
              onClick={() => setSelected(video)}
            >
              <PencilIcon />
            </Button>
            {video.isDelete ? (
              <Button
                type="button"
                size="icon-sm"
                variant="secondary"
                aria-label="恢复视频"
                title="恢复"
                disabled={status.isPending}
                onClick={() => status.mutate({ id: video.id, deleted: false })}
              >
                {status.isPending ? <Spinner /> : <RefreshCcwIcon />}
              </Button>
            ) : (
              <ConfirmAction
                trigger={
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="destructive"
                    aria-label="删除视频"
                    title="删除"
                  >
                    <Trash2Icon />
                  </Button>
                }
                title="删除这个视频？"
                description="视频只会被标记为删除，原有弹幕和第三方弹幕池关联仍会保留"
                destructive
                pending={status.isPending}
                onConfirm={() => status.mutate({ id: video.id, deleted: true })}
              />
            )}
          </div>
        ),
      },
    ],
    [status],
  );

  function search(event: FormEvent) {
    event.preventDefault();
    setPage(1);
    setQuery(draftQuery.trim());
  }

  function submitCreate(event: FormEvent) {
    event.preventDefault();
    create.mutate(
      { vid: createVid.trim(), name: createName.trim() },
      {
        onSuccess: () => {
          setCreateOpen(false);
          setCreateVid("");
          setCreateName("");
        },
      },
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="视频与弹幕库"
        title="视频管理"
        description="管理视频名称、状态以及系统和第三方弹幕池关联"
      />
      <Card>
        <CardHeader>
          <CardTitle>视频</CardTitle>
          <CardDescription>
            外部弹幕请求会自动创建仅包含视频 ID 的记录，名称可在此补充
          </CardDescription>
          <CardAction>
            <Button type="button" onClick={() => setCreateOpen(true)}>
              <PlusIcon data-icon="inline-start" />
              添加视频
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          <form onSubmit={search}>
            <FieldGroup>
              <Field orientation="horizontal">
                <Input
                  aria-label="搜索视频"
                  value={draftQuery}
                  placeholder="搜索视频名称或视频 ID"
                  onChange={(event) => setDraftQuery(event.target.value)}
                />
                <SearchableSelect
                  options={[
                    { value: "all", label: "全部状态" },
                    { value: "false", label: "正常" },
                    { value: "true", label: "已删除" },
                  ]}
                  value={deleted}
                  onValueChange={(value) => {
                    setDeleted(value);
                    setPage(1);
                  }}
                  className="sm:w-36"
                  searchPlaceholder="搜索状态"
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
          {videos.isError ? (
            <div className="p-6">
              <QueryError
                error={videos.error}
                retry={() => void videos.refetch()}
              />
            </div>
          ) : videos.isPending ? (
            <LoadingTable />
          ) : (
            <DataTable
              rows={videos.data?.list ?? []}
              columns={columns}
              rowKey={(video) => String(video.id)}
              emptyTitle="暂无视频"
              emptyDescription="添加视频，或通过外部弹幕接口自动创建"
            />
          )}
        </CardContent>
        {videos.data ? (
          <ListPagination
            meta={{ page, pageSize: 20, total: videos.data.total }}
            onPageChange={setPage}
          />
        ) : null}
      </Card>
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <form className="flex flex-col gap-5" onSubmit={submitCreate}>
            <DialogHeader>
              <DialogTitle>添加视频</DialogTitle>
              <DialogDescription>
                视频 ID 创建后不可修改，名称可以留空
              </DialogDescription>
            </DialogHeader>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="create-video-vid">视频 ID</FieldLabel>
                <Input
                  id="create-video-vid"
                  value={createVid}
                  maxLength={36}
                  required
                  onChange={(event) => setCreateVid(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="create-video-name">视频名称</FieldLabel>
                <Input
                  id="create-video-name"
                  value={createName}
                  maxLength={200}
                  placeholder="可选"
                  onChange={(event) => setCreateName(event.target.value)}
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
                创建
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <VideoDialog
        video={selected}
        bilibiliPools={bilibiliPools.data ?? []}
        iqiyiPools={iqiyiPools.data ?? []}
        dandanplayPools={dandanplayPools.data ?? []}
        externalPools={externalPools.data ?? []}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      />
    </div>
  );
}

function VideoDialog({
  video,
  bilibiliPools,
  iqiyiPools,
  dandanplayPools,
  externalPools,
  onOpenChange,
}: {
  video: ManagedVideo | null;
  bilibiliPools: BilibiliPool[];
  iqiyiPools: IqiyiPool[];
  dandanplayPools: DandanplayPool[];
  externalPools: ExternalPool[];
  onOpenChange: (open: boolean) => void;
}) {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [source, setSource] = useState<"bilibili" | "iqiyi" | "dandanplay" | "external">("bilibili");
  const [poolID, setPoolID] = useState("");
  const [offset, setOffset] = useState("0");
  const [exportFormat, setExportFormat] = useState("danuni.json");
  const detail = useQuery({
    queryKey: ["video", video?.id],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<ManagedVideo>>(
          `/api/admin/videos/${video?.id}`,
        )
      ).data,
    enabled: Boolean(video),
  });
  const heatmap = useQuery({
    queryKey: ["video-heatmap", video?.id],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<HeatmapPoint[]>>(
          `/api/admin/videos/${video?.id}/heatmap`,
        )
      ).data,
    enabled: Boolean(video),
  });
  useEffect(() => {
    setName(detail.data?.name ?? video?.name ?? "");
  }, [detail.data?.name, video]);
  const save = useApiMutation<{ name: string }, ApiResponse<ManagedVideo>>({
    mutationFn: (body) => apiPut(`/api/admin/videos/${video?.id}`, body),
    successMessage: "视频名称已保存",
    invalidate: [["videos"], ["video"]],
  });
  const bind = useApiMutation<
    { source: "bilibili" | "iqiyi" | "dandanplay" | "external"; poolId: number | string; offset: number },
    ApiResponse<BilibiliBinding | IqiyiBinding | DandanplayBinding | ExternalBinding>
  >({
    mutationFn: ({ source: targetSource, ...body }) =>
      apiPost(`/api/admin/videos/${video?.id}/${targetSource}-bindings`, body),
    successMessage: "弹幕池关联已保存",
    invalidate: [
      ["videos"],
      ["video"],
      ["video-heatmap"],
      ["bilibili-pools"],
      ["iqiyi-pools"],
      ["dandanplay-pools"],
      ["external-pools"],
    ],
  });
  const remove = useApiMutation<
    { source: "bilibili" | "iqiyi" | "dandanplay" | "external"; id: number },
    ApiResponse<null>
  >({
    mutationFn: ({ source: targetSource, id }) =>
      apiDelete(
        `/api/admin/videos/${video?.id}/${targetSource}-bindings/${id}`,
      ),
    successMessage: "弹幕池关联已删除",
    invalidate: [
      ["videos"],
      ["video"],
      ["video-heatmap"],
      ["bilibili-pools"],
      ["iqiyi-pools"],
      ["dandanplay-pools"],
      ["external-pools"],
    ],
  });
  type ThirdPartyBinding =
    | { source: "bilibili"; binding: BilibiliBinding }
    | { source: "iqiyi"; binding: IqiyiBinding }
    | { source: "dandanplay"; binding: DandanplayBinding }
    | { source: "external"; binding: ExternalBinding };
  const bindings: ThirdPartyBinding[] = [
    ...(detail.data?.bilibiliBindings ?? []).map((binding) => ({
      source: "bilibili" as const,
      binding,
    })),
    ...(detail.data?.iqiyiBindings ?? []).map((binding) => ({
      source: "iqiyi" as const,
      binding,
    })),
    ...(detail.data?.dandanplayBindings ?? []).map((binding) => ({
      source: "dandanplay" as const,
      binding,
    })),
    ...(detail.data?.externalBindings ?? []).map((binding) => ({
      source: "external" as const,
      binding,
    })),
  ];
  const columns: DataColumn<ThirdPartyBinding>[] = [
    {
      key: "source",
      label: "来源",
      render: (item) => (
        <Badge variant="outline">
          {item.source === "bilibili"
            ? "bilibili"
            : item.source === "iqiyi"
              ? "爱奇艺"
              : item.source === "dandanplay" ? "弹弹play" : "外部导入"}
        </Badge>
      ),
    },
    {
      key: "pool",
      label: "第三方弹幕池",
      render: (item) => (
        <div>
          <p className="font-medium">
            {item.source === "bilibili"
              ? poolLabel(item.binding)
              : item.source === "iqiyi"
                ? item.binding.poolVid
                : item.source === "dandanplay" ? dandanplayPoolLabel({episodeId: item.binding.poolEpisodeId, withRelated: item.binding.withRelated}) : item.binding.poolName}
          </p>
          {item.source === "bilibili" ? (
            <p className="font-mono text-xs text-muted-foreground">
              CID {item.binding.cid}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      key: "offset",
      label: "时间偏移",
      render: (item) => (
        <Badge variant="outline">{offsetLabel(item.binding.offset)}</Badge>
      ),
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
              aria-label={`删除 ${item.source} 弹幕池关联`}
              title="删除关联"
              disabled={detail.data?.isDelete}
            >
              <Trash2Icon />
            </Button>
          }
          title="删除这个弹幕池关联？"
          description="删除后，这个第三方弹幕池会停止合并到视频弹幕"
          destructive
          pending={remove.isPending}
          onConfirm={() =>
            remove.mutate({ source: item.source, id: item.binding.id })
          }
        />
      ),
    },
  ];

  function submitName(event: FormEvent) {
    event.preventDefault();
    save.mutate({ name: name.trim() });
  }

  function submitBinding(event: FormEvent) {
    event.preventDefault();
    bind.mutate(
      {
        source,
        poolId: source === "external" ? poolID : Number(poolID),
        offset: Number(offset),
      },
      {
        onSuccess: () => {
          setPoolID("");
          setOffset("0");
        },
      },
    );
  }

  return (
    <Dialog open={Boolean(video)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{detail.data?.name || video?.name || "视频详情"}</DialogTitle>
          <DialogDescription className="font-mono">
            {video?.vid}
          </DialogDescription>
        </DialogHeader>
        {detail.isError ? (
          <QueryError error={detail.error} retry={() => void detail.refetch()} />
        ) : detail.isPending ? (
          <LoadingTable />
        ) : (
          <div className="flex flex-col gap-4">
            <Card>
              <CardHeader>
                <CardTitle>基本信息</CardTitle>
                <CardDescription>
                  视频 ID 不可修改，视频名称可以留空
                </CardDescription>
              </CardHeader>
              <CardContent>
                <form onSubmit={submitName}>
                  <FieldGroup>
                    <Field orientation="horizontal">
                      <Input
                        aria-label="视频名称"
                        value={name}
                        maxLength={200}
                        placeholder="未命名视频"
                        onChange={(event) => setName(event.target.value)}
                      />
                      <Button type="submit" disabled={save.isPending}>
                        {save.isPending ? (
                          <Spinner data-icon="inline-start" />
                        ) : null}
                        保存名称
                      </Button>
                    </Field>
                  </FieldGroup>
                </form>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>弹幕热力图</CardTitle>
                <CardDescription>
                  按秒统计系统弹幕与全部已关联第三方弹幕，峰值越高表示该时段越密集
                </CardDescription>
              </CardHeader>
              <CardContent>
                {heatmap.isError ? (
                  <QueryError error={heatmap.error} retry={() => void heatmap.refetch()} />
                ) : heatmap.isPending ? (
                  <div className="grid h-48 place-items-center"><Spinner /></div>
                ) : (heatmap.data?.length ?? 0) > 0 ? (
                  <ChartContainer
                    className="h-48 w-full"
                    config={{ count: { label: "弹幕数", color: "var(--chart-1)" } } satisfies ChartConfig}
                  >
                    <AreaChart data={heatmap.data} margin={{ left: 4, right: 4 }}>
                      <CartesianGrid vertical={false} />
                      <XAxis
                        dataKey="time"
                        tickLine={false}
                        axisLine={false}
                        minTickGap={28}
                        tickFormatter={(value) => formatDuration(Number(value))}
                      />
                      <ChartTooltip
                        cursor={false}
                        content={
                          <ChartTooltipContent
                            labelFormatter={(_, payload) =>
                              payload?.[0]
                                ? formatDuration(Number(payload[0].payload.time))
                                : ""
                            }
                          />
                        }
                      />
                      <Area
                        dataKey="count"
                        type="monotone"
                        fill="var(--color-count)"
                        fillOpacity={0.28}
                        stroke="var(--color-count)"
                        strokeWidth={1.5}
                      />
                    </AreaChart>
                  </ChartContainer>
                ) : (
                  <p className="py-12 text-center text-sm text-muted-foreground">
                    暂无弹幕，添加或关联弹幕池后会显示热力图
                  </p>
                )}
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>系统弹幕池</CardTitle>
                <CardDescription>
                  管理这个视频由系统直接接收的弹幕
                </CardDescription>
                <CardAction>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      if (!video) return;
                      onOpenChange(false);
                      navigate(`/danmaku?vid=${encodeURIComponent(video.vid)}`);
                    }}
                  >
                    <SearchIcon data-icon="inline-start" />
                    管理弹幕
                  </Button>
                </CardAction>
              </CardHeader>
              <CardContent>
                <div className="mb-4 flex items-center justify-between gap-3">
                  <Badge variant="secondary">{detail.data?.danmakuCount ?? 0} 条弹幕</Badge>
                  {detail.data?.isDelete ? <span className="text-xs text-muted-foreground">已删除的视频不能发送弹幕</span> : null}
                </div>
                <AddDanmakuForm vid={video?.vid} disabled={detail.data?.isDelete} />
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>第三方弹幕池</CardTitle>
                <CardDescription>
                  可关联 bilibili、爱奇艺或外部导入弹幕池；偏移量为正时延后、为负时提前
                </CardDescription>
              </CardHeader>
              <CardContent>
                <form onSubmit={submitBinding}>
                  <FieldGroup className="grid gap-4 md:grid-cols-3">
                    <Field data-disabled={detail.data?.isDelete}>
                      <FieldLabel htmlFor="video-binding-source">
                        弹幕来源
                      </FieldLabel>
                      <SearchableSelect
                        id="video-binding-source"
                        options={[
                          { value: "bilibili", label: "bilibili" },
                          { value: "iqiyi", label: "爱奇艺" },
                          { value: "dandanplay", label: "弹弹play" },
                          { value: "external", label: "外部导入" },
                        ]}
                        value={source}
                        disabled={detail.data?.isDelete}
                        searchPlaceholder="搜索弹幕来源"
                        onValueChange={(value) => {
                          setSource(value as "bilibili" | "iqiyi" | "dandanplay" | "external");
                          setPoolID("");
                        }}
                      />
                    </Field>
                    <Field data-disabled={detail.data?.isDelete}>
                      <FieldLabel htmlFor="video-binding-pool">
                        弹幕池
                      </FieldLabel>
                      <SearchableSelect
                        id="video-binding-pool"
                        options={
                          source === "bilibili"
                            ? bilibiliPools.map((pool) => ({
                                value: String(pool.id),
                                label: poolLabel(pool),
                              }))
                            : source === "iqiyi"
                              ? iqiyiPools.map((pool) => ({
                                  value: String(pool.id),
                                  label: pool.vid,
                                }))
                              : source === "dandanplay" ? dandanplayPools.map((pool) => ({value: String(pool.id),label: dandanplayPoolLabel(pool)}))
                              : externalPools.map((pool) => ({
                                  value: pool.id,
                                  label: `${pool.name} · ${pool.id}`,
                                }))
                        }
                        value={poolID}
                        disabled={detail.data?.isDelete}
                        placeholder={`选择${source === "bilibili" ? " bilibili" : source === "iqiyi" ? "爱奇艺" : source === "dandanplay" ? "弹弹play" : "外部导入"}弹幕池`}
                        searchPlaceholder="搜索弹幕池"
                        onValueChange={setPoolID}
                      />
                    </Field>
                    <Field data-disabled={detail.data?.isDelete}>
                      <FieldLabel htmlFor="video-binding-offset">
                        偏移量（秒）
                      </FieldLabel>
                      <div className="flex gap-2">
                        <Input
                          id="video-binding-offset"
                          type="number"
                          step="any"
                          value={offset}
                          disabled={detail.data?.isDelete}
                          required
                          onChange={(event) => setOffset(event.target.value)}
                        />
                        <Button
                          type="submit"
                          disabled={
                            bind.isPending ||
                            !poolID ||
                            detail.data?.isDelete
                          }
                        >
                          {bind.isPending ? (
                            <Spinner data-icon="inline-start" />
                          ) : (
                            <LinkIcon data-icon="inline-start" />
                          )}
                          关联
                        </Button>
                      </div>
                      {detail.data?.isDelete ? (
                        <FieldDescription>
                          请先恢复视频，再修改弹幕池关联
                        </FieldDescription>
                      ) : null}
                    </Field>
                  </FieldGroup>
                </form>
              </CardContent>
              <CardContent className="p-0">
                <DataTable
                  rows={bindings}
                  columns={columns}
                  rowKey={(item) => `${item.source}-${item.binding.id}`}
                  emptyTitle="暂无第三方弹幕池"
                  emptyDescription="选择来源和弹幕池后即可关联"
                />
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>导出弹幕</CardTitle>
                <CardDescription>
                  导出当前视频已合并的系统与第三方弹幕，可额外通过 API 设置 offset
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Field orientation="horizontal">
                  <SearchableSelect
                    options={exportFormats}
                    value={exportFormat}
                    searchPlaceholder="搜索输出格式"
                    onValueChange={setExportFormat}
                  />
                  <Button
                    render={
                      <a
                        href={`/api/danmaku/export?id=${encodeURIComponent(video?.vid ?? "")}&format=${encodeURIComponent(exportFormat)}`}
                      />
                    }
                  >
                    <DownloadIcon data-icon="inline-start" />
                    下载
                  </Button>
                </Field>
              </CardContent>
            </Card>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
