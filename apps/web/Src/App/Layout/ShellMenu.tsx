import { preloadFrame } from "@/App/Layout/LazyFrames";
import { OrganizationChip } from "@/App/Layout/OrganizationChip";
import type { TFrameId } from "@/App/Panels";
import { useShellNavigation } from "@/App/Layout/useShellNavigation";
import { canReadAudit, canReadRunners } from "@/Core/Auth/AuthTypes";
import { useCurrentMembership, useMe } from "@/Core/Auth/useMe";
import { useSignOut } from "@/Core/Auth/useSignOut";
import { Mark } from "@/Core/Ui/Mark";
import { SideMenu, type IMenuItem, type TMenuEntry } from "@/Core/Ui/SideMenu";
import { useT } from "@/I18n/useT";

const intentFor = (id: TFrameId) => () => {
  void preloadFrame(id);
};

const stackItems =
  (stack: TFrameId[], openStack: (ids: TFrameId[]) => void) =>
  (
    frame: TFrameId,
    icon: IMenuItem["icon"],
    label: string,
    current = stack.includes(frame),
  ): IMenuItem => ({
    id: frame,
    icon,
    label,
    href: `/?stack=${frame}`,
    current,
    onSelect: () => {
      openStack([frame]);
    },
    onIntent: intentFor(frame),
  });

export const ShellMenu = () => {
  const t = useT();
  const { stack, goWorkspace, openStack } = useShellNavigation();

  const me = useMe();
  const { SignOut, Pending } = useSignOut();
  const role = useCurrentMembership()?.Role;

  const item = stackItems(stack, openStack);
  const items: TMenuEntry[] = [
    {
      id: "home",
      icon: "home",
      label: t("shell.nav.home"),
      href: "/",
      current: stack.length === 0,
      onSelect: goWorkspace,
    },
    {
      id: "projects",
      icon: "layers",
      label: t("shell.nav.projects"),
      hint: t("shell.soon"),
    },
    item(
      "jobs",
      "play",
      t("shell.nav.jobs"),
      stack.some((frame) => frame === "jobs" || frame.startsWith("job-")),
    ),
    { ...item("organization", "users", t("shell.nav.team")), id: "team" },
  ];
  if (canReadRunners(role)) {
    items.splice(3, 0, item("runners", "server", t("shell.nav.runners")));
  }
  if (canReadAudit(role)) {
    items.push(item("audit", "file", t("shell.nav.audit")));
  }
  const bottom: IMenuItem[] = [
    {
      ...item("appearance", "settings", t("shell.nav.settings")),
      id: "settings",
    },
  ];

  return (
    <SideMenu
      label={t("shell.nav.label")}
      brand={
        <div className="grid min-w-0 gap-2.5">
          <span className="flex items-center gap-2.5">
            <Mark size={28} />
            <span className="text-title-xl">{t("shell.brand")}</span>
          </span>
          <OrganizationChip />
        </div>
      }
      items={items}
      bottom={bottom}
      account={{
        id: "account",
        name: me.data?.User.Name ?? "",
        label: me.data?.User.Name ?? t("shell.nav.account"),
        href: "/?stack=account",
        current: stack.includes("account"),
        onSelect: () => {
          openStack(["account"]);
        },
        onIntent: intentFor("account"),
        signOut: {
          label: t("account.signOut"),
          busy: Pending,
          onSelect: () => {
            void SignOut();
          },
        },
      }}
    />
  );
};
