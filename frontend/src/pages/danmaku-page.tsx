import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import {
  EraserIcon,
  PlusIcon,
  PencilIcon,
  RotateCcwIcon,
  SearchIcon,
  Trash2Icon,
} from "lucide-react";

import { apiGet, apiPost } from "@/api/client";
import type { ApiResponse, Danmaku } from "@/api/types";
import { ConfirmAction } from "@/components/confirm-action";
import { AddDanmakuForm } from "@/components/add-danmaku-form";
import { NativeRulePanel } from "@/components/native-rule-panel";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DataTable, type DataColumn } from "@/components/data-table";
import { ListPagination } from "@/components/list-pagination";
import { LoadingTable } from "@/components/loading-table";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { SearchableSelect } from "@/components/searchable-select";
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
} from "@/components/ui/dialog";
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useApiMutation } from "@/hooks/use-api-mutation";
import { colorHex, formatDateTime } from "@/lib/format";

const modes = [
  { value: "all", label: "全部类型" },
  { value: "1", label: "滚动弹幕" },
  { value: "4", label: "底部弹幕" },
  { value: "5", label: "顶部弹幕" },
  { value: "8", label: "高级弹幕" },
  { value: "9", label: "特殊弹幕" },
];
const editableModes = modes.slice(1);
type Filters = {
  vid: string;
  startDate: string;
  endDate: string;
  mode: string;
  ip: string;
  key: string;
  descending: string;
};
const emptyFilters: Filters = {
  vid: "",
  startDate: "",
  endDate: "",
  mode: "all",
  ip: "",
  key: "",
  descending: "true",
};

function apiDate(value: string): string {
  if (!value) return "";
  const normalized = value.replace("T", " ");
  return normalized.length === 16 ? `${normalized}:00` : normalized;
}

function modeLabel(mode: number): string {
  return (
    editableModes.find((item) => item.value === String(mode))?.label ??
    `类型 ${mode}`
  );
}

export function DanmakuPage() {
  const [adding, setAdding] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();
  const initialFilters = { ...emptyFilters, vid: searchParams.get("vid") ?? "" };
  const [draft, setDraft] = useState<Filters>(initialFilters);
  const [filters, setFilters] = useState<Filters>(initialFilters);
  const [page, setPage] = useState(1);
  const [editingID, setEditingID] = useState<string | null>(null);
  const vids = useQuery({
    queryKey: ["danmaku-vids"],
    queryFn: async () =>
      (await apiGet<ApiResponse<string[]>>("/api/admin/danmakulist/vids")).data,
  });
  const danmaku = useQuery({
    queryKey: ["danmaku", page, filters],
    queryFn: async () =>
      (
        await apiGet<ApiResponse<{ total: number; list: Danmaku[] }>>(
          "/api/admin/danmakulist/baseselect",
          {
            page,
            size: 20,
            ...filters,
            startDate: apiDate(filters.startDate),
            endDate: apiDate(filters.endDate),
            mode: filters.mode === "all" ? "" : filters.mode,
            descending: filters.descending === "true",
          },
        )
      ).data,
  });
  const remove = useApiMutation<string, ApiResponse<null>>({
    mutationFn: (id) => apiGet("/api/admin/danmakuedit/delete", { id }),
    successMessage: "弹幕已删除",
    invalidate: [["danmaku"]],
  });
  const columns = useMemo<DataColumn<Danmaku>[]>(
    () => [
      {
        key: "content",
        label: "弹幕内容",
        render: (item) => (
          <div className="max-w-md">
            <Button
              type="button"
              variant="link"
              className="h-auto max-w-full justify-start p-0 text-left font-medium"
              title={item.data.text ?? ""}
              onClick={() => setEditingID(item.id)}
            >
              <span className="block max-w-full truncate">
                {item.data.text || "—"}
              </span>
            </Button>
            <p className="text-xs text-muted-foreground">
              {modeLabel(item.data.mode)} · {item.data.time.toFixed(2)} 秒
            </p>
          </div>
        ),
      },
      {
        key: "video",
        label: "视频 ID",
        render: (item) => (
          <p className="max-w-56 truncate font-mono text-xs" title={item.vid}>
            {item.vid}
          </p>
        ),
      },
      {
        key: "ip",
        label: "IP 地址",
        className: "whitespace-nowrap",
        render: (item) => (
          <span className="font-mono text-xs">{item.ip || "—"}</span>
        ),
      },
      {
        key: "status",
        label: "状态",
        render: (item) => (
          <Badge variant={item.isDelete ? "destructive" : "secondary"}>
            {item.isDelete ? "已删除" : "正常"}
          </Badge>
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
        className: "w-20 whitespace-nowrap text-right",
        render: (item) => (
          <div className="flex items-center justify-end gap-1">
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label="查看并编辑弹幕"
              title="查看并编辑"
              onClick={() => setEditingID(item.id)}
            >
              <PencilIcon />
            </Button>
            <ConfirmAction
              trigger={
                <Button
                  type="button"
                  size="icon-sm"
                  variant="destructive"
                  aria-label="删除弹幕"
                  title="删除"
                >
                  <Trash2Icon />
                </Button>
              }
              title="删除这条弹幕？"
              description="弹幕会被标记为已删除，并立即从公开查询结果中隐藏。"
              destructive
              pending={remove.isPending}
              onConfirm={() => remove.mutate(item.id)}
            />
          </div>
        ),
      },
    ],
    [remove],
  );

  function submit(event: FormEvent) {
    event.preventDefault();
    setPage(1);
    setFilters({ ...draft });
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      if (draft.vid) next.set("vid", draft.vid);
      else next.delete("vid");
      return next;
    });
  }
  function reset() {
    setDraft(emptyFilters);
    setFilters(emptyFilters);
    setPage(1);
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      next.delete("vid");
      return next;
    });
  }

  const data = danmaku.data;
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Moderation"
        title="弹幕管理"
        description="快速检索、审阅并维护所有播放器产生的弹幕数据。"
        action={
          <div className="flex items-center gap-3">
            {data ? <Badge variant="outline">共 {data.total} 条</Badge> : null}
            <Button onClick={() => setAdding(true)}><PlusIcon data-icon="inline-start" />添加弹幕</Button>
          </div>
        }
      />
      <Dialog open={adding} onOpenChange={setAdding}>
        <DialogContent>
          <DialogHeader><DialogTitle>添加弹幕</DialogTitle><DialogDescription>向指定视频的系统弹幕池添加弹幕。</DialogDescription></DialogHeader>
          {adding ? <AddDanmakuForm initialVid={filters.vid} onSuccess={() => setAdding(false)} /> : null}
        </DialogContent>
      </Dialog>
      <Tabs defaultValue="danmaku">
        <TabsList className="h-auto flex-wrap">
          <TabsTrigger value="danmaku">弹幕</TabsTrigger>
          <TabsTrigger value="keywords">关键词过滤</TabsTrigger>
          <TabsTrigger value="authors">用户名映射</TabsTrigger>
          <TabsTrigger value="ips">IP 黑名单</TabsTrigger>
        </TabsList>
        <TabsContent value="keywords"><NativeRulePanel kind="keywords" /></TabsContent>
        <TabsContent value="authors"><NativeRulePanel kind="authors" /></TabsContent>
        <TabsContent value="ips"><NativeRulePanel kind="ips" /></TabsContent>
        <TabsContent value="danmaku" className="flex flex-col gap-6">
      <Card>
        <CardContent>
          <form className="flex flex-col gap-4" onSubmit={submit}>
            <FieldGroup className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Field>
                <FieldLabel htmlFor="filter-vid">视频 ID</FieldLabel>
                <SearchableSelect
                  id="filter-vid"
                  options={[
                    { value: "", label: "全部视频" },
                    ...(vids.data ?? []).map((vid) => ({
                      value: vid,
                      label: vid,
                    })),
                  ]}
                  value={draft.vid}
                  placeholder="全部视频"
                  searchPlaceholder="搜索视频 ID"
                  onValueChange={(value) =>
                    setDraft({ ...draft, vid: value })
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="filter-key">内容关键词</FieldLabel>
                <Input
                  id="filter-key"
                  value={draft.key}
                  placeholder="搜索弹幕内容"
                  onChange={(event) =>
                    setDraft({ ...draft, key: event.target.value })
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="filter-ip">IP 地址</FieldLabel>
                <Input
                  id="filter-ip"
                  value={draft.ip}
                  placeholder="IPv4 或 IPv6"
                  onChange={(event) =>
                    setDraft({ ...draft, ip: event.target.value })
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="filter-mode">弹幕类型</FieldLabel>
                <SearchableSelect
                  id="filter-mode"
                  options={modes}
                  value={draft.mode}
                  searchPlaceholder="搜索弹幕类型"
                  onValueChange={(value) =>
                    setDraft({ ...draft, mode: value })
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="filter-start">开始时间</FieldLabel>
                <Input
                  id="filter-start"
                  type="datetime-local"
                  value={draft.startDate}
                  onChange={(event) =>
                    setDraft({ ...draft, startDate: event.target.value })
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="filter-end">结束时间</FieldLabel>
                <Input
                  id="filter-end"
                  type="datetime-local"
                  value={draft.endDate}
                  onChange={(event) =>
                    setDraft({ ...draft, endDate: event.target.value })
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="filter-order">排序</FieldLabel>
                <SearchableSelect
                  id="filter-order"
                  options={[
                    { value: "true", label: "最新在前" },
                    { value: "false", label: "最早在前" },
                  ]}
                  value={draft.descending}
                  searchPlaceholder="搜索排序方式"
                  onValueChange={(value) =>
                    setDraft({ ...draft, descending: value })
                  }
                />
              </Field>
            </FieldGroup>
            <div className="flex flex-wrap gap-2">
              <Button type="submit">
                <SearchIcon data-icon="inline-start" />
                查询
              </Button>
              <Button type="button" variant="outline" onClick={reset}>
                <RotateCcwIcon data-icon="inline-start" />
                重置
              </Button>
              {filters.key ||
              filters.vid ||
              filters.ip ||
              filters.startDate ||
              filters.endDate ||
              filters.mode !== "all" ? (
                <Badge variant="secondary">
                  <EraserIcon />
                  已应用筛选
                </Badge>
              ) : null}
            </div>
          </form>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="p-0">
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
              rows={data?.list ?? []}
              columns={columns}
              rowKey={(item) => item.id}
              emptyTitle="没有匹配的弹幕"
              emptyDescription="请调整筛选条件后重新查询。"
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
        </TabsContent>
      </Tabs>
      <DanmakuEditor
        id={editingID}
        onOpenChange={(open) => {
          if (!open) setEditingID(null);
        }}
      />
    </div>
  );
}

function DanmakuEditor({
  id,
  onOpenChange,
}: {
  id: string | null;
  onOpenChange: (open: boolean) => void;
}) {
  const detail = useQuery({
    queryKey: ["danmaku-detail", id],
    queryFn: async () =>
      (await apiGet<ApiResponse<Danmaku>>("/api/admin/danmakuedit", { id }))
        .data,
    enabled: Boolean(id),
  });
  const [time, setTime] = useState("0");
  const [mode, setMode] = useState("1");
  const [size, setSize] = useState("25");
  const [color, setColor] = useState("#ffffff");
  const [text, setText] = useState("");
  const [deleted, setDeleted] = useState(false);
  const save = useApiMutation<Record<string, unknown>, ApiResponse<Danmaku>>({
    mutationFn: (body) => apiPost("/api/admin/danmakuedit/edit", body),
    successMessage: "弹幕已更新",
    invalidate: [["danmaku"], ["danmaku-detail", id]],
  });

  useEffect(() => {
    if (!detail.data) return;
    const item = detail.data;
    setTime(String(item.data.time));
    setMode(String(item.data.mode));
    setSize(String(item.data.size));
    setColor(colorHex(item.data.color));
    setText(item.data.text ?? "");
    setDeleted(item.isDelete);
  }, [detail.data]);

  const item = detail.data;
  return (
    <Dialog open={Boolean(id)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-2xl">
        <form
          className="flex flex-col gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (!item) return;
            save.mutate(
              {
                id: item.id,
                isDelete: deleted,
                data: {
                  ...item.data,
                  time: Number(time),
                  mode: Number(mode),
                  size: Number(size),
                  color: Number.parseInt(color.slice(1), 16),
                  text,
                },
              },
              { onSuccess: () => onOpenChange(false) },
            );
          }}
        >
          <DialogHeader>
            <DialogTitle>查看并编辑弹幕</DialogTitle>
            <DialogDescription>
              修改展示参数、内容和软删除状态。
            </DialogDescription>
          </DialogHeader>
          {detail.isError ? (
            <QueryError
              error={detail.error}
              retry={() => void detail.refetch()}
            />
          ) : detail.isPending || !item ? (
            <LoadingTable rows={6} />
          ) : (
            <FieldGroup>
              <Field>
                <FieldLabel>记录 ID</FieldLabel>
                <Input value={item.id} readOnly />
              </Field>
              <Field>
                <FieldLabel>视频 ID</FieldLabel>
                <Input value={item.vid} readOnly />
              </Field>
              <Field>
                <FieldLabel>IP 地址</FieldLabel>
                <Input value={item.ip || "—"} readOnly />
              </Field>
              <FieldGroup className="grid gap-4 sm:grid-cols-3">
                <Field>
                  <FieldLabel htmlFor="danmaku-time">出现时间（秒）</FieldLabel>
                  <Input
                    id="danmaku-time"
                    type="number"
                    min="0"
                    step="any"
                    value={time}
                    required
                    onChange={(event) => setTime(event.target.value)}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="danmaku-mode">类型</FieldLabel>
                  <SearchableSelect
                    id="danmaku-mode"
                    options={editableModes}
                    value={mode}
                    searchPlaceholder="搜索弹幕类型"
                    onValueChange={setMode}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="danmaku-size">字号</FieldLabel>
                  <Input
                    id="danmaku-size"
                    type="number"
                    min="1"
                    step="1"
                    value={size}
                    required
                    onChange={(event) => setSize(event.target.value)}
                  />
                </Field>
              </FieldGroup>
              <Field>
                <FieldLabel htmlFor="danmaku-color">颜色</FieldLabel>
                <div className="flex gap-3">
                  <Input
                    id="danmaku-color"
                    type="color"
                    className="w-16 p-1"
                    value={color}
                    onChange={(event) => setColor(event.target.value)}
                  />
                  <Input
                    aria-label="颜色十六进制值"
                    value={color}
                    pattern="^#[0-9a-fA-F]{6}$"
                    onChange={(event) => setColor(event.target.value)}
                  />
                </div>
              </Field>
              <Field>
                <FieldLabel htmlFor="danmaku-text">弹幕内容</FieldLabel>
                <Textarea
                  id="danmaku-text"
                  rows={4}
                  value={text}
                  required
                  onChange={(event) => setText(event.target.value)}
                />
                <FieldDescription>
                  公开查询会返回这里保存的文本。
                </FieldDescription>
              </Field>
              <Field
                orientation="horizontal"
                className="w-full justify-between rounded-xl border bg-muted/30 px-4 py-3"
              >
                <FieldContent>
                  <FieldLabel htmlFor="danmaku-deleted">
                    标记为已删除
                  </FieldLabel>
                  <FieldDescription>
                    启用后，公开播放器接口将隐藏此弹幕。
                  </FieldDescription>
                </FieldContent>
                <Switch
                  id="danmaku-deleted"
                  checked={deleted}
                  onCheckedChange={setDeleted}
                />
              </Field>
              <p className="text-right text-xs text-muted-foreground">
                创建：{formatDateTime(item.createTime)} · 最后修改：
                {formatDateTime(item.updateTime)}
              </p>
            </FieldGroup>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button type="submit" disabled={save.isPending || !item}>
              {save.isPending ? <Spinner data-icon="inline-start" /> : null}
              保存修改
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
