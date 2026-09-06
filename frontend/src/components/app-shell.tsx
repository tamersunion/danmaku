import { useEffect } from "react";
import {
  ClapperboardIcon,
  DatabaseIcon,
  EllipsisVerticalIcon,
  ListFilterIcon,
  LogOutIcon,
  UploadIcon,
  UserRoundIcon,
  UsersIcon,
} from "lucide-react";
import { NavLink, Outlet, useLocation } from "react-router-dom";

import { useSession } from "@/auth/session-context";
import { DanmakuLogo } from "@/components/danmaku-logo";
import { ThemeToggle, ThemeMenuItems } from "@/components/theme-toggle";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Separator } from "@/components/ui/separator";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
  useSidebar,
} from "@/components/ui/sidebar";
import { initials, sessionRoleLabel } from "@/lib/format";

const titles: Array<[RegExp, string]> = [
  [/^\/videos/, "视频管理"],
  [/^\/danmaku/, "弹幕管理"],
  [/^\/bilibili/, "bilibili"],
  [/^\/iqiyi/, "爱奇艺"],
  [/^\/animeko/, "Animeko"],
  [/^\/bahamut/, "巴哈姆特"],
  [/^\/tencent/, "腾讯视频"],
  [/^\/youku/, "优酷"],
  [/^\/dandanplay/, "弹弹play"],
  [/^\/external/, "外部导入"],
  [/^\/users/, "用户管理"],
  [/^\/profile/, "个人资料"],
];

export function AppShell() {
  return (
    <SidebarProvider>
      <AppShellContent />
    </SidebarProvider>
  );
}

function AppShellContent() {
  const session = useSession();
  const location = useLocation();
  const { setOpenMobile } = useSidebar();

  useEffect(() => {
    setOpenMobile(false);
  }, [location.key, location.pathname, setOpenMobile]);
  const title =
    titles.find(([pattern]) => pattern.test(location.pathname))?.[1] ??
    "弹幕控制台";
  const logoutPath =
    session.provider === "cas" ? "/cas/logout" : "/api/admin/logout";

  return (
    <>
      <Sidebar collapsible="icon" variant="inset" className="md:z-30">
        <SidebarHeader className="p-3 group-data-[collapsible=icon]:p-2">
          <NavLink
            to="/"
            className="flex h-10 items-center gap-2.5 overflow-hidden rounded-lg px-1.5 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0"
          >
            <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground shadow-sm">
              <DanmakuLogo className="size-[1.4rem]" />
            </span>
            <span className="min-w-0 group-data-[collapsible=icon]:hidden">
              <span className="block truncate text-sm font-semibold">
                弹幕控制台
              </span>
            </span>
          </NavLink>
        </SidebarHeader>
        <SidebarContent>
          {session.canManageDanmaku ? (
            <SidebarGroup>
              <SidebarGroupLabel>视频与弹幕库</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="视频管理"
                      isActive={location.pathname.startsWith("/videos")}
                      render={<NavLink to="/videos" />}
                    >
                      <ClapperboardIcon />
                      <span>视频管理</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="弹幕管理"
                      isActive={location.pathname.startsWith("/danmaku")}
                      render={<NavLink to="/danmaku" />}
                    >
                      <ListFilterIcon />
                      <span>弹幕管理</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          ) : null}
          {session.canManageDanmaku ? (
            <SidebarGroup>
              <SidebarGroupLabel>第三方弹幕库</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="bilibili"
                      isActive={location.pathname.startsWith("/bilibili")}
                      render={<NavLink to="/bilibili" />}
                    >
                      <DatabaseIcon />
                      <span>bilibili</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="爱奇艺"
                      isActive={location.pathname.startsWith("/iqiyi")}
                      render={<NavLink to="/iqiyi" />}
                    >
                      <DatabaseIcon />
                      <span>爱奇艺</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
<SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="弹弹play"
                      isActive={location.pathname.startsWith("/dandanplay")}
                      render={<NavLink to="/dandanplay" />}
                    >
                      <DatabaseIcon />
                      <span>弹弹play</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
<SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="Animeko"
                      isActive={location.pathname.startsWith("/animeko")}
                      render={<NavLink to="/animeko" />}
                    >
                      <DatabaseIcon />
                      <span>Animeko</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
<SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="巴哈姆特"
                      isActive={location.pathname.startsWith("/bahamut")}
                      render={<NavLink to="/bahamut" />}
                    >
                      <DatabaseIcon />
                      <span>巴哈姆特</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
<SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="腾讯视频"
                      isActive={location.pathname.startsWith("/tencent")}
                      render={<NavLink to="/tencent" />}
                    >
                      <DatabaseIcon />
                      <span>腾讯视频</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
<SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="优酷"
                      isActive={location.pathname.startsWith("/youku")}
                      render={<NavLink to="/youku" />}
                    >
                      <DatabaseIcon />
                      <span>优酷</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="外部导入"
                      isActive={location.pathname.startsWith("/external")}
                      render={<NavLink to="/external" />}
                    >
                      <UploadIcon />
                      <span>外部导入</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          ) : null}
          {session.canManageUsers ? (
            <SidebarGroup>
              <SidebarGroupLabel>平台管理</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      tooltip="用户管理"
                      isActive={location.pathname.startsWith("/users")}
                      render={<NavLink to="/users" />}
                    >
                      <UsersIcon />
                      <span>用户管理</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          ) : null}
        </SidebarContent>
        <SidebarFooter className="p-3">
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <button className="flex w-full items-center gap-2 rounded-lg p-2 text-left outline-none transition-colors hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-sidebar-ring group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:p-0" />
              }
            >
              <Avatar size="sm">
                {session.avatar ? (
                  <AvatarImage src={session.avatar} alt="" />
                ) : null}
                <AvatarFallback>{initials(session.name)}</AvatarFallback>
              </Avatar>
              <span className="min-w-0 flex-1 group-data-[collapsible=icon]:hidden">
                <span className="block truncate text-xs font-medium">
                  {session.name}
                </span>
                <span className="block truncate text-xs text-muted-foreground">
                  {session.email || session.username}
                </span>
              </span>
              <EllipsisVerticalIcon className="shrink-0 text-muted-foreground group-data-[collapsible=icon]:hidden" />
            </DropdownMenuTrigger>
            <DropdownMenuContent side="right" align="end" className="min-w-56">
              <DropdownMenuGroup>
                <DropdownMenuLabel className="flex items-center justify-between gap-2">
                  <span>{session.name}</span>
                  <Badge
                    variant={
                      session.role === "GeneralUser" ? "secondary" : "default"
                    }
                  >
                    {sessionRoleLabel(session.role)}
                  </Badge>
                </DropdownMenuLabel>
                <DropdownMenuItem render={<NavLink to="/profile" />}>
                  <UserRoundIcon />
                  个人资料
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <ThemeMenuItems />
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuItem
                  variant="destructive"
                  render={<a href={logoutPath} />}
                >
                  <LogOutIcon />
                  退出登录
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>
      <SidebarInset className="md:!m-0 md:!rounded-none md:!shadow-none">
        <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center gap-3 border-b bg-background/90 px-4 backdrop-blur md:px-6">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="my-auto h-4" />
          <span className="text-sm font-medium">{title}</span>
          <ThemeToggle />
        </header>
        <main className="flex-1 bg-muted/20 px-4 py-6 md:px-6 md:py-8">
          <div className="mx-auto w-full max-w-[1500px]">
            <Outlet />
          </div>
        </main>
      </SidebarInset>
    </>
  );
}
