import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  LinkIcon,
  PencilIcon,
  PlusIcon,
  RefreshCcwIcon,
  SearchIcon,
  Trash2Icon,
} from "lucide-react";

import { apiDelete, apiGet, apiPatch, apiPost, apiPut } from "@/api/client";
import type {
  ApiResponse,
  BilibiliBinding,
  BilibiliPool,
  ManagedVideo,
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
import { useApiMutation } from "@/hooks/use-api-mutation";
import { formatDateTime } from "@/lib/format";

type Paged<T> = { total: number; list: T[] };

function poolLabel(pool: Pick<BilibiliPool, "bvid" | "cid" | "p">): string {
  return pool.bvid
    ? `${pool.bvid} / P${pool.p}`
    : `CID ${pool.cid} / P${pool.p}`;
}

function offsetLabel(offset: number): string {
  return `${offset > 0 ? "+" : ""}${offset} 秒`;
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
  const pools = useQuery({
    queryKey: ["bilibili-pool-options"],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<BilibiliPool>>>(
          "/api/admin/bilibili/pools",
          { page: 1, size: 500 },
        )
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
          <Button
            type="button"
            variant="link"
            className="h-auto max-w-80 justify-start p-0 text-left"
            onClick={() => setSelected(video)}
          >
            <span className="truncate">{video.name || "未命名视频"}</span>
          </Button>
        ),
      },
      {
        key: "vid",
        label: "视频 ID",
        render: (video) => (
          <span className="font-mono text-xs">{video.vid}</span>
        ),
      },
      {
        key: "pools",
        label: "弹幕池",
        render: (video) => (
          <div className="flex flex-wrap gap-2">
            <Badge variant="secondary">系统弹幕 {video.danmakuCount} 条</Badge>
            <Badge variant="outline">
              bilibili {video.bilibiliPoolCount} 个
            </Badge>
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
              variant="ghost"
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
                variant="outline"
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
                description="视频只会被标记为删除，原有弹幕和第三方弹幕池关联仍会保留。"
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
        eyebrow="Library"
        title="视频管理"
        description="管理视频名称、状态以及系统和第三方弹幕池关联。"
      />
      <Card>
        <CardHeader>
          <CardTitle>视频</CardTitle>
          <CardDescription>
            外部弹幕请求会自动创建仅包含视频 ID 的记录，名称可在此补充。
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
                <Select
                  items={[
                    { value: "all", label: "全部状态" },
                    { value: "false", label: "正常" },
                    { value: "true", label: "已删除" },
                  ]}
                  value={deleted}
                  onValueChange={(value) => {
                    setDeleted(value ?? "false");
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
                      <SelectItem value="true">已删除</SelectItem>
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
              emptyDescription="添加视频，或通过外部弹幕接口自动创建。"
            />
          )}
          {videos.data ? (
            <ListPagination
              meta={{ page, pageSize: 20, total: videos.data.total }}
              onPageChange={setPage}
            />
          ) : null}
        </CardContent>
      </Card>
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <form className="flex flex-col gap-5" onSubmit={submitCreate}>
            <DialogHeader>
              <DialogTitle>添加视频</DialogTitle>
              <DialogDescription>
                视频 ID 创建后不可修改，名称可以留空。
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
        pools={pools.data ?? []}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      />
    </div>
  );
}

function VideoDialog({
  video,
  pools,
  onOpenChange,
}: {
  video: ManagedVideo | null;
  pools: BilibiliPool[];
  onOpenChange: (open: boolean) => void;
}) {
  const [name, setName] = useState("");
  const [poolID, setPoolID] = useState("");
  const [offset, setOffset] = useState("0");
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
  useEffect(() => {
    setName(detail.data?.name ?? video?.name ?? "");
  }, [detail.data?.name, video]);
  const save = useApiMutation<{ name: string }, ApiResponse<ManagedVideo>>({
    mutationFn: (body) => apiPut(`/api/admin/videos/${video?.id}`, body),
    successMessage: "视频名称已保存",
    invalidate: [["videos"], ["video"]],
  });
  const bind = useApiMutation<
    { poolId: number; offset: number },
    ApiResponse<BilibiliBinding>
  >({
    mutationFn: (body) =>
      apiPost(`/api/admin/videos/${video?.id}/bilibili-bindings`, body),
    successMessage: "弹幕池关联已保存",
    invalidate: [["videos"], ["video"], ["bilibili-pools"]],
  });
  const remove = useApiMutation<number, ApiResponse<null>>({
    mutationFn: (bindingID) =>
      apiDelete(
        `/api/admin/videos/${video?.id}/bilibili-bindings/${bindingID}`,
      ),
    successMessage: "弹幕池关联已删除",
    invalidate: [["videos"], ["video"], ["bilibili-pools"]],
  });
  const bindings = detail.data?.bilibiliBindings ?? [];
  const columns: DataColumn<BilibiliBinding>[] = [
    {
      key: "pool",
      label: "bilibili 弹幕池",
      render: (binding) => (
        <div>
          <p className="font-medium">{poolLabel(binding)}</p>
          <p className="font-mono text-xs text-muted-foreground">
            CID {binding.cid}
          </p>
        </div>
      ),
    },
    {
      key: "offset",
      label: "时间偏移",
      render: (binding) => (
        <Badge variant="outline">{offsetLabel(binding.offset)}</Badge>
      ),
    },
    {
      key: "actions",
      label: "操作",
      className: "w-16 text-right",
      render: (binding) => (
        <ConfirmAction
          trigger={
            <Button
              type="button"
              size="icon-sm"
              variant="destructive"
              aria-label="删除 bilibili 弹幕池关联"
              title="删除关联"
              disabled={detail.data?.isDelete}
            >
              <Trash2Icon />
            </Button>
          }
          title="删除这个弹幕池关联？"
          description="系统自带弹幕池不受影响，只有这个 bilibili 弹幕池会停止合并。"
          destructive
          pending={remove.isPending}
          onConfirm={() => remove.mutate(binding.id)}
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
      { poolId: Number(poolID), offset: Number(offset) },
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
                  视频 ID 不可修改，视频名称可以留空。
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
                <CardTitle>系统自带弹幕池</CardTitle>
                <CardDescription>
                  每个视频固定关联系统自带弹幕池，不能取消。
                </CardDescription>
                <CardAction>
                  <Switch
                    aria-label="系统自带弹幕池固定关联"
                    checked
                    disabled
                  />
                </CardAction>
              </CardHeader>
              <CardContent>
                <Badge variant="secondary">
                  {detail.data?.danmakuCount ?? 0} 条弹幕
                </Badge>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>bilibili 弹幕池</CardTitle>
                <CardDescription>
                  可关联多个弹幕池，偏移量为正时延后、为负时提前。
                </CardDescription>
              </CardHeader>
              <CardContent>
                <form onSubmit={submitBinding}>
                  <FieldGroup className="grid gap-4 md:grid-cols-2">
                    <Field data-disabled={detail.data?.isDelete}>
                      <FieldLabel htmlFor="video-binding-pool">
                        弹幕池
                      </FieldLabel>
                      <Select
                        items={pools.map((pool) => ({
                          value: String(pool.id),
                          label: poolLabel(pool),
                        }))}
                        value={poolID}
                        disabled={detail.data?.isDelete}
                        onValueChange={(value) => setPoolID(value ?? "")}
                      >
                        <SelectTrigger
                          id="video-binding-pool"
                          className="w-full"
                        >
                          <SelectValue placeholder="选择 bilibili 弹幕池" />
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
                          请先恢复视频，再修改弹幕池关联。
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
                  rowKey={(binding) => String(binding.id)}
                  emptyTitle="暂无第三方弹幕池"
                  emptyDescription="系统自带弹幕池仍保持固定关联。"
                />
              </CardContent>
            </Card>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
