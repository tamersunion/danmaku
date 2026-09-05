import { useId, useState, type FormEvent } from "react";
import { SendIcon } from "lucide-react";
import { apiPost } from "@/api/client";
import type { ApiResponse } from "@/api/types";
import { SearchableSelect } from "@/components/searchable-select";
import { Button } from "@/components/ui/button";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { useApiMutation } from "@/hooks/use-api-mutation";

export function AddDanmakuForm({ vid, initialVid = "", disabled = false, onSuccess }: {
 vid?: string; initialVid?: string; disabled?: boolean; onSuccess?: () => void;
}) {
 const prefix = useId();
 const [videoID, setVideoID] = useState(initialVid);
 const [time, setTime] = useState("0");
 const [type, setType] = useState("0");
 const [color, setColor] = useState("#ffffff");
 const [text, setText] = useState("");
 const mutation = useApiMutation<{id:string;time:number;type:number;color:number;text:string;author:string;referer:string},ApiResponse<null>>({
  mutationFn: body => apiPost("/api/danmaku/dplayer/v3",body),
  successMessage: "弹幕已添加",
  invalidate: [["danmaku"],["danmaku-vids"],["videos"],["video"],["video-heatmap"]],
 });
 function submit(event:FormEvent) {
  event.preventDefault();
  mutation.mutate({id:vid ?? videoID.trim(),time:Number(time),type:Number(type),color:Number.parseInt(color.slice(1),16),text:text.trim(),author:"",referer:window.location.href}, {
   onSuccess:()=>{setText("");onSuccess?.();},
  });
 }
 return <form onSubmit={submit}>
  <FieldGroup>
   {vid === undefined ? <Field><FieldLabel htmlFor={`${prefix}-vid`}>视频 ID</FieldLabel><Input id={`${prefix}-vid`} value={videoID} onChange={e=>setVideoID(e.target.value)} maxLength={36} required disabled={mutation.isPending} /></Field> : null}
   <FieldGroup className="grid gap-4 sm:grid-cols-3">
    <Field data-disabled={disabled}><FieldLabel htmlFor={`${prefix}-time`}>出现时间（秒）</FieldLabel><Input id={`${prefix}-time`} type="number" min="0" step="any" value={time} onChange={e=>setTime(e.target.value)} required disabled={disabled} /></Field>
    <Field data-disabled={disabled}><FieldLabel htmlFor={`${prefix}-type`}>类型</FieldLabel><SearchableSelect id={`${prefix}-type`} value={type} onValueChange={setType} disabled={disabled} searchPlaceholder="搜索类型" options={[{value:"0",label:"滚动"},{value:"1",label:"顶部"},{value:"2",label:"底部"}]} /></Field>
    <Field data-disabled={disabled}><FieldLabel htmlFor={`${prefix}-color`}>颜色</FieldLabel><Input id={`${prefix}-color`} type="color" value={color} onChange={e=>setColor(e.target.value)} disabled={disabled} className="p-1 [&::-webkit-color-swatch-wrapper]:p-0 [&::-webkit-color-swatch]:rounded-sm [&::-webkit-color-swatch]:border-0" /></Field>
   </FieldGroup>
   <Field data-disabled={disabled}><FieldLabel htmlFor={`${prefix}-text`}>弹幕内容</FieldLabel><Input id={`${prefix}-text`} value={text} onChange={e=>setText(e.target.value)} maxLength={500} required disabled={disabled} /></Field>
   <Button type="submit" className="self-end" disabled={disabled || mutation.isPending || !text.trim() || !(vid ?? videoID.trim())}>{mutation.isPending ? <Spinner data-icon="inline-start"/> : <SendIcon data-icon="inline-start"/>}添加弹幕</Button>
  </FieldGroup>
 </form>;
}
