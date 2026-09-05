import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  EyeIcon,
  FileUpIcon,
  LinkIcon,
  RefreshCwIcon,
  SearchIcon,
} from "lucide-react";

import { apiGet, apiPost, apiPut } from "@/api/client";
import type {
  ApiResponse,
  ExternalBinding,
  ExternalPool,
  ExternalPoolDanmaku,
  ManagedVideo,
} from "@/api/types";
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
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { useApiMutation } from "@/hooks/use-api-mutation";
import {
  importDanmakuFile,
  importFormats,
  type ImportFormat,
} from "@/lib/danmaku-import";
import { formatDateTime } from "@/lib/format";

type Paged<T> = { total: number; list: T[] };

export function ExternalPage() {
  const [page, setPage] = useState(1);
  const [draftQuery, setDraftQuery] = useState("");
  const [query, setQuery] = useState("");
  const [importPool, setImportPool] = useState<ExternalPool | null | undefined>();
  const [detailPool, setDetailPool] = useState<ExternalPool | null>(null);
  const [bindingPool, setBindingPool] = useState<ExternalPool | null>(null);
  const pools = useQuery({
    queryKey: ["external-pools", page, query],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<ExternalPool>>>(
          "/api/admin/external",
          { page, size: 20, query },
        )
      ).data,
  });
  const columns = useMemo<DataColumn<ExternalPool>[]>(
    () => [
      {
        key: "pool",
        label: "弹幕池",
        render: (pool) => (
          <div>
            <p className="font-medium">{pool.name}</p>
            <p className="font-mono text-xs text-muted-foreground">
              {pool.id}
            </p>
          </div>
        ),
      },
      {
        key: "format",
        label: "导入格式",
        render: (pool) => <Badge variant="outline">{pool.sourceFormat}</Badge>,
      },
      {
        key: "count",
        label: "弹幕",
        render: (pool) => <Badge variant="secondary">{pool.danmakuCount} 条</Badge>,
      },
      {
        key: "binding",
        label: "关联",
        render: (pool) => `${pool.bindingCount} 个视频 ID`,
      },
      {
        key: "updated",
        label: "最后导入",
        className: "whitespace-nowrap",
        render: (pool) => formatDateTime(pool.updateTime),
      },
      {
        key: "actions",
        label: "操作",
        className: "w-28 whitespace-nowrap text-right",
        render: (pool) => (
          <div className="flex justify-end gap-1">
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label="查看外部弹幕池"
              title="查看弹幕"
              onClick={() => setDetailPool(pool)}
            >
              <EyeIcon />
            </Button>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label="重新导入"
              title="重新导入"
              onClick={() => setImportPool(pool)}
            >
              <RefreshCwIcon />
            </Button>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label="关联视频"
              title="关联视频"
              onClick={() => setBindingPool(pool)}
            >
              <LinkIcon />
            </Button>
          </div>
        ),
      },
    ],
    [],
  );

  function search(event: FormEvent) {
    event.preventDefault();
    setPage(1);
    setQuery(draftQuery.trim());
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Library"
        title="外部导入"
        description="将不同平台或播放器格式统一导入为可关联的视频弹幕池。"
      />
      <Card>
        <CardHeader>
          <CardTitle>弹幕池</CardTitle>
          <CardDescription>
            每个弹幕池使用独立 ID；重新导入会原子覆盖池内原有弹幕。
          </CardDescription>
          <CardAction>
            <Button type="button" onClick={() => setImportPool(null)}>
              <FileUpIcon data-icon="inline-start" />
              导入弹幕池
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          <form onSubmit={search}>
            <Field orientation="horizontal">
              <Input
                value={draftQuery}
                placeholder="搜索名称、弹幕池 ID 或格式"
                aria-label="搜索外部弹幕池"
                onChange={(event) => setDraftQuery(event.target.value)}
              />
              <Button type="submit" variant="outline">
                <SearchIcon data-icon="inline-start" />
                查询
              </Button>
            </Field>
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
              rowKey={(pool) => pool.id}
              emptyTitle="暂无外部弹幕池"
              emptyDescription="导入一个支持格式的弹幕文件后即可关联视频。"
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
      <ImportDialog
        pool={importPool}
        open={importPool !== undefined}
        onOpenChange={(open) => {
          if (!open) setImportPool(undefined);
        }}
      />
      <ExternalDetailDialog pool={detailPool} onOpenChange={(open) => !open && setDetailPool(null)} />
      <ExternalBindingDialog pool={bindingPool} onOpenChange={(open) => !open && setBindingPool(null)} />
    </div>
  );
}

function ImportDialog({
  pool,
  open,
  onOpenChange,
}: {
  pool: ExternalPool | null | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [name, setName] = useState("");
  const [format, setFormat] = useState<ImportFormat>("common.json");
  const [file, setFile] = useState<File | null>(null);
  const [parseError, setParseError] = useState("");
  const [parsing, setParsing] = useState(false);
  useEffect(() => {
    if (!open) return;
    setName(pool?.name ?? "");
    setFile(null);
    setParseError("");
    if (pool && importFormats.some((option) => option.value === pool.sourceFormat)) {
      setFormat(pool.sourceFormat as ImportFormat);
    } else {
      setFormat("common.json");
    }
  }, [open, pool]);
  const mutation = useApiMutation<
    { name: string; sourceFormat: string; danmaku: Awaited<ReturnType<typeof importDanmakuFile>> },
    ApiResponse<ExternalPool>
  >({
    mutationFn: (body) =>
      pool
        ? apiPut(`/api/admin/external/${pool.id}`, body)
        : apiPost("/api/admin/external", body),
    successMessage: pool ? "弹幕池已覆盖导入" : "弹幕池已导入",
    invalidate: [["external-pools"], ["external-pool-options"], ["videos"], ["video"]],
  });

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!file) return;
    setParsing(true);
    setParseError("");
    try {
      const danmaku = await importDanmakuFile(file, format);
      mutation.mutate(
        { name: name.trim() || pool?.name || file.name, sourceFormat: format, danmaku },
        {
          onSuccess: () => {
            onOpenChange(false);
            setName("");
            setFile(null);
          },
        },
      );
    } catch (error) {
      setParseError(error instanceof Error ? error.message : "文件解析失败");
    } finally {
      setParsing(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form className="flex flex-col gap-5" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{pool ? "重新导入弹幕池" : "导入外部弹幕池"}</DialogTitle>
            <DialogDescription>
              {pool ? "新文件解析成功后会完整覆盖旧弹幕，弹幕池 ID 与视频关联保持不变。" : "选择来源格式后在浏览器内统一解析，再保存为新的弹幕池。"}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="external-pool-name">名称</FieldLabel>
              <Input
                id="external-pool-name"
                value={name}
                maxLength={200}
                placeholder={pool?.name || file?.name || "弹幕池名称"}
                onChange={(event) => setName(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="external-pool-format">文件格式</FieldLabel>
              <SearchableSelect
                id="external-pool-format"
                options={[...importFormats]}
                value={format}
                searchPlaceholder="搜索输入格式"
                onValueChange={(value) => setFormat(value as ImportFormat)}
              />
            </Field>
            <Field data-invalid={Boolean(parseError)}>
              <FieldLabel htmlFor="external-pool-file">弹幕文件</FieldLabel>
              <Input
                id="external-pool-file"
                type="file"
                required
                aria-invalid={Boolean(parseError)}
                onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              />
              {parseError ? <p className="text-sm text-destructive">{parseError}</p> : null}
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              取消
            </Button>
            <Button type="submit" disabled={!file || parsing || mutation.isPending}>
              {parsing || mutation.isPending ? <Spinner data-icon="inline-start" /> : <FileUpIcon data-icon="inline-start" />}
              {pool ? "覆盖导入" : "开始导入"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ExternalDetailDialog({ pool, onOpenChange }: { pool: ExternalPool | null; onOpenChange: (open: boolean) => void }) {
  const [page, setPage] = useState(1);
  const danmaku = useQuery({
    queryKey: ["external-pool-danmaku", pool?.id, page],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<ExternalPoolDanmaku>>>(`/api/admin/external/${pool?.id}/danmaku`, { page, size: 20 })
      ).data,
    enabled: Boolean(pool),
  });
  const columns: DataColumn<ExternalPoolDanmaku>[] = [
    { key: "time", label: "出现时间", render: (item) => `${item.data.time.toFixed(3)} 秒` },
    { key: "content", label: "内容", render: (item) => <span className="line-clamp-2 max-w-xl">{item.data.text}</span> },
  ];
  return (
    <Dialog open={Boolean(pool)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{pool?.name}</DialogTitle>
          <DialogDescription className="font-mono">{pool?.id}</DialogDescription>
        </DialogHeader>
        <div className="overflow-hidden rounded-xl border">
          {danmaku.isError ? <div className="p-6"><QueryError error={danmaku.error} retry={() => void danmaku.refetch()} /></div> : danmaku.isPending ? <LoadingTable /> : <DataTable rows={danmaku.data?.list ?? []} columns={columns} rowKey={(item) => String(item.id)} emptyTitle="这个弹幕池为空" emptyDescription="可通过重新导入写入弹幕。" />}
          {danmaku.data ? <ListPagination variant="inline" meta={{ page, pageSize: 20, total: danmaku.data.total }} onPageChange={setPage} /> : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function ExternalBindingDialog({ pool, onOpenChange }: { pool: ExternalPool | null; onOpenChange: (open: boolean) => void }) {
  const [videoID, setVideoID] = useState("");
  const [offset, setOffset] = useState("0");
  const videos = useQuery({
    queryKey: ["external-binding-video-options"],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<Paged<ManagedVideo>>>("/api/admin/videos", { page: 1, size: 500, deleted: false })
      ).data.list,
    enabled: Boolean(pool),
  });
  const bind = useApiMutation<{ videoId: number; poolId: string; offset: number }, ApiResponse<ExternalBinding>>({
    mutationFn: ({ videoId, ...body }) => apiPost(`/api/admin/videos/${videoId}/external-bindings`, body),
    successMessage: "弹幕池已关联到视频",
    invalidate: [["external-pools"], ["videos"], ["video"]],
  });
  function submit(event: FormEvent) {
    event.preventDefault();
    if (!pool) return;
    bind.mutate(
      { videoId: Number(videoID), poolId: pool.id, offset: Number(offset) },
      { onSuccess: () => onOpenChange(false) },
    );
  }
  return (
    <Dialog open={Boolean(pool)} onOpenChange={onOpenChange}>
      <DialogContent>
        <form className="flex flex-col gap-5" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>关联视频</DialogTitle>
            <DialogDescription>将“{pool?.name}”合并到指定视频，偏移量单位为秒。</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="external-binding-video">视频</FieldLabel>
              <SearchableSelect
                id="external-binding-video"
                options={(videos.data ?? []).map((video) => ({ value: String(video.id), label: `${video.name || "未命名视频"} · ${video.vid}` }))}
                value={videoID}
                searchPlaceholder="搜索视频名称或 ID"
                onValueChange={setVideoID}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="external-binding-offset">偏移量（秒）</FieldLabel>
              <Input id="external-binding-offset" type="number" step="any" value={offset} required onChange={(event) => setOffset(event.target.value)} />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
            <Button type="submit" disabled={!videoID || bind.isPending}>
              {bind.isPending ? <Spinner data-icon="inline-start" /> : <LinkIcon data-icon="inline-start" />}
              关联
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
