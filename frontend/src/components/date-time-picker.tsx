import { useState } from "react";
import { CalendarIcon } from "lucide-react";
import { zhCN } from "react-day-picker/locale";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTitle, PopoverTrigger } from "@/components/ui/popover";
import { SearchableSelect } from "@/components/searchable-select";
import { localDateValue, parseLocalDate } from "@/lib/picker-values";

export function DateTimePicker({ id, value, onChange }: { id: string; value: string; onChange: (value: string) => void }) {
  const [open, setOpen] = useState(false);
  const selected = parseLocalDate(value);
  const [month, setMonth] = useState(selected ?? new Date());
  const time = value.split("T")[1] || "00:00:00";
  const select = (date: Date | undefined) => { if (date) onChange(`${localDateValue(date)}T${time}`); };
  return <Popover open={open} onOpenChange={next => { setOpen(next); if (next) setMonth(selected ?? new Date()); }}>
    <PopoverTrigger render={<Button id={id} type="button" variant="outline" className="w-full justify-start" />}>
      <CalendarIcon data-icon="inline-start" /><span className="truncate">{value ? value.replace("T", " ") : "选择日期和时间"}</span>
    </PopoverTrigger>
    <PopoverContent align="start" className="w-80 max-w-[calc(100vw-2rem)] gap-3 p-4">
      <PopoverTitle>选择日期和时间</PopoverTitle>
      <FieldGroup className="grid grid-cols-2 gap-2">
        <Field><FieldLabel className="sr-only" htmlFor={`${id}-year`}>年份</FieldLabel><SearchableSelect id={`${id}-year`} value={String(month.getFullYear())} onValueChange={year => setMonth(new Date(Number(year), month.getMonth(), 1))} searchPlaceholder="搜索年份" options={Array.from({length: Math.max(new Date().getFullYear() + 10, month.getFullYear()) - 1900 + 1}, (_, i) => ({value: String(1900+i), label: `${1900+i} 年`}))} /></Field>
        <Field><FieldLabel className="sr-only" htmlFor={`${id}-month`}>月份</FieldLabel><SearchableSelect id={`${id}-month`} value={String(month.getMonth())} onValueChange={next => setMonth(new Date(month.getFullYear(), Number(next), 1))} searchPlaceholder="搜索月份" options={Array.from({length: 12}, (_, i) => ({value: String(i), label: `${i+1} 月`}))} /></Field>
      </FieldGroup>
      <Calendar mode="single" locale={zhCN} weekStartsOn={1} selected={selected} onSelect={select} month={month} onMonthChange={setMonth} className="w-full p-0 [--cell-size:--spacing(9)]" />
      <Field><FieldLabel htmlFor={`${id}-time`}>时间（时:分:秒）</FieldLabel><Input id={`${id}-time`} type="time" step="1" value={time} disabled={!selected} onChange={event => { if (selected && event.target.value) onChange(`${localDateValue(selected)}T${event.target.value}`); }} /></Field>
      <div className="flex justify-between gap-2">
        <Button type="button" variant="ghost" onClick={() => { onChange(""); setOpen(false); }}>清空</Button>
        <Button type="button" onClick={() => setOpen(false)}>完成</Button>
      </div>
    </PopoverContent>
  </Popover>;
}
