# ADR-0055: تنفيذ الـ sandbox في M3.6: driver الـ Podman، والـ agent كـ PID 1، وقناة التحكم، والـ seccomp، والـ escape والـ limits suites

- **الحالة:** مقبول (قرار مفوَّض في M3.6)
- **التاريخ:** 2026-10-28
- **بيكمّل:** [ADR-0043](0043-job-sandbox-rootless-podman.md) و[ADR-0045](0045-secrets-transport-and-masking.md) و[ADR-0053](0053-runner-implementation.md).

## السياق
ADR-0043 حدد الـ sandbox: Podman rootless، وagent هو PID 1، وحدود cgroup v2 بتفشل مقفولة، وseccomp بقايمة منع، و`inspect` بعد الـ create. M3.6 هو أول تنفيذ حقيقي له، والتنفيذ قفل أسئلة: الـ agent يعيش فين في الكود، وقناة التحكم شكلها إيه، والإعدادات تتكتب فين، وإزاي نثبت إن الحاوية معزولة فعلًا على جهاز حقيقي، وفين يشتغل الإثبات ده.

## القرار

### طبقة `Agent`
- طبقة جديدة في `apps/runner/internal/Agent`، بتستورد `Sandbox` بس (عشان `Wire` والثوابت)، ومحدش بيستوردها غير `App`. الـ hook (`runner_rules.py`) بيفرض ده.
- **الـ agent هو نفس binary الـ runner**، متركّب read-only على `/run/dhow/agent` وبيتشغّل بـ `__agent` كـ entrypoint. فمفيش binary تاني يتنزّل أو يتوقّع، والـ agent دايمًا نفس نسخة الـ runner. وده معناه إن الـ binary لازم يتبني static (`CGO_ENABLED=0`، زي الـ release).
- أول حاجة بيعملها `PR_SET_DUMPABLE=0`، فمفيش process في الـ job يقدر يعمل ptrace عليه أو يقرا `/proc/1/environ` أو `/proc/1/root` من غير `CAP_SYS_PTRACE`، والـ capability دي مش موجودة.
- كل خطوة `/bin/sh -e -s` والسكريبت على stdin (مش على القرص ولا في argv)، في process group لوحدها، و`SIGTERM` و`SIGKILL` بيروحوا للـ group كله.
- ملفات الأسرار في `/run/dhow/secrets` (tmpfs بـ 4 MiB و`noexec,nosuid,nodev` و`0700`)، بـ `O_NOFOLLOW` و`0400`، وبتتمسح بعد الخطوة.
- لو القناة اتقفلت (الـ runner مات أو قتل الحاوية) الـ agent بيخرج، والحاوية بتموت معاه لأنه PID 1.

### قناة التحكم (`Sandbox/Wire`)
- unix socket في مجلد خاص بالـ attempt (`<state>/jobs/<attempt>/control`) متركّب read-only على `/run/dhow/control`. الـ runner هو اللي بيسمع والـ agent هو اللي بيتصل.
- **الـ runner بيقفل الـ listener بعد أول اتصال**، فأي process في الـ job بيحاول يتصل بعد كده بياخد `ECONNREFUSED` ومايقدرش ينتحل الـ agent. والـ escape suite بيثبت ده.
- الإطار: طول 4 bytes بعده رسالة `agentv1.Frame` بـ protobuf، وحد 1 MiB للإطار، وأول رسالة `Hello` برقم البروتوكول (1)، وأي رقم تاني بيقفل القناة.

### الإعدادات والتخزين
- ملفات `containers.conf` و`storage.conf` و`registries.conf` والـ seccomp بتتكتب **في مجلد الحالة** (`<state>/containers/`) عند كل إقلاع، مش في `/etc/dhow-runner/containers/` زي ما ADR-0043 كان كاتب. السبب: الـ runner بيكتبها بنفسه بصلاحيات مستخدمه، ومحدش بيعدّلها بإيده، فمكانها جنب الـ storage. الـ short names مقفولة، والـ cgroup manager `cgroupfs`، والـ runtime `crun`.
- **`SkipPull`:** خيار في الـ driver بيستخدم image موجودة في الـ storage بدل الـ pull (`podman image exists`). مش متوصّل بأي إعداد دلوقتي، والـ isolation suite بس هو اللي بيستخدمه، لأن الـ image بتاعته متبنية محليًا ومالهاش registry. لو احتجناه للأجهزة اللي من غير إنترنت، بيتوصّل بمتغير بيئة بقرار.
- **`Podman(ctx, args...)`:** بيشغّل Podman بإعدادات الـ driver نفسها. الـ suite بيستخدمه للـ build والـ inspect والـ grep جوه الـ storage، و`dhow-runner doctor` (M3.10) هيستخدمه.

### الـ seccomp
- `Seccomp.json` متضمّن في الـ binary، وبيمنع بـ `EPERM` كل syscall في قايمة ADR-0043، وكمان الـ mount API الجديد (`fsopen` و`fsconfig` و`fsmount` و`fspick` و`move_mount` و`open_tree` و`mount_setattr`، و`umount`) عشان مايبقاش بديل لـ `mount()`. اختبار وحدة بيقارن الملف بالقايمتين.

### إثبات العزل: `apps/runner/Test/Isolation`
- **`escape-probe`:** برنامج Go static بيشتغل كـ job جوه حاوية حقيقية، وبيجرّب كل طريق هروب في قايمة M3-Runner (قراية ملفات الـ host ومجلد الـ runner، و`/proc/1/root` و`/proc/1/environ`، والـ mounts، والكتابة في `/proc/sys` و`/sys` والـ cgroup، وقناة التحكم، والـ capabilities، و`clone(CLONE_NEWUSER)` و`mount` و`add_key` و`bpf` و`io_uring_setup` و`perf_event_open` وptrace على الـ agent والـ raw sockets و`init_module`، والـ sockets بتاعة Docker وPodman، وcanary من الـ runner في أي ملف أو environ). كل probe لازم يطلع `blocked`، وأي `ESCAPED` بيفشّل الاختبار.
- **الـ probe بيرفض يشتغل برّه الـ sandbox** (من غير `/run/dhow/agent` بيخرج بـ 4)، لأن نجاح الـ probes على جهاز عادي بيغيّر الجهاز فعلًا: أول تجربة يدوية كـ root برّه الحاوية ركّبت tmpfs على `/tmp` وغيّرت الـ hostname.
- **الـ image:** `FROM scratch` فيها `busybox` static كـ `/bin/sh` والـ probe ونسخة setuid منه، بتتبني بـ `podman build --network=none` جوه storage الـ driver.
- **الـ limits suite:** fork bomb بيقف بـ `EAGAIN` عند `pids`، والذاكرة بتخلص `OOM_KILLED`، والـ CPU بيتعمله throttling (`cpu.stat`)، وملء `/workspace` بيدّي `ENOSPC`، و`SIGTERM` بيقفل الخطوة و`SIGKILL` بيقفل اللي بيتجاهله، و64 MiB وسطر 10 MiB بيوصلوا كاملين، و`Cleanup` بيشيل كل حاويات الـ runner.
- **jobين مع بعض:** كل واحد بيحط علامة في `/workspace` و`/tmp` و`/dev/shm` وabstract socket، وبيدوّر على علامة التاني فيهم وفي `/proc`.
- **الأسرار:** خطوة فيها سر في الـ env وفي ملف، وبعدها السر مش موجود في `podman inspect` ولا في أي ملف تحت مجلد الحالة (`podman unshare grep -r`، يعني الـ storage كمان)، وملفات الأسرار اتمسحت.
- **مش في M3.6:** probes الشبكة والـ proxy (M3.7)، والإلغاء بالتدريج الكامل و`timed_out` والفوضى (M3.9). والـ masking والـ logs اتختبروا في M3.5 بالـ driver الوهمي.
- **`make runner-isolation`** بيبني الـ runner والـ probe static وبيبني الاختبار بـ tag `isolation`، و`ISOLATION_RUN` بيسمح يشتغل تحت `systemd-run`.

### job الـ `isolation` في الـ CI
- **قرار Sabry:** على GitHub Actions بـ `ubuntu-24.04`، مش VM. بيشتغل على أي PR مش draft يلمس `apps/runner` أو `packages/proto` أو `internal/jobspec` أو الـ `Makefile` أو الـ workflow، وعلى كل push لـ `main`.
- بيتأكد إن Podman 5.4 أو أحدث، وإلا بيفشل برسالة واضحة.
- user مخصص (`dhowci`) بمليون subuid وsubgid، عشان كل job ليه userns بـ 65536 والـ suite بيشغّل jobين مع بعض.
- `kernel.apparmor_restrict_unprivileged_userns=0`، لأن Ubuntu 24.04 بيمنع الـ user namespaces لأي حد مش متعرّف في AppArmor. ده لازم يتكتب في دليل التنصيب لـ Ubuntu 24.04.
- الاختبار بيشتغل تحت `systemd-run --uid=dhowci -p Delegate=yes -p MemorySwapMax=0`، زي الـ service الحقيقية بالظبط.

## العواقب
- العزل بيتثبت على جهاز حقيقي في كل PR يلمس الـ runner، وأي probe بينجح بيوقف الـ merge.
- **نضحّي** بالتحقق المحلي لصالح الـ CI: الـ suite محتاج cgroup v2 وتفويض وsubuids وPodman 5.4، ومش كل جهاز تطوير فيه ده. والـ CI مابيشتغلش على الـ draft، فالتحقق بيبدأ لما الـ PR يبقى Ready for review.
- `/proc/self/mountinfo` جوه الـ job بيبيّن المسار على الـ host للـ mounts بتاعة الـ agent وقناة التحكم (المسار بس، من غير المحتوى). ده مقبول، والـ probe بيتأكد إن مفيش mount غير المسموح وإن الاتنين دول read-only.
- لو Podman اللي على `ubuntu-24.04` أقدم من 5.4، الـ job هيفشل، والحل تثبيت نسخة مثبّتة بـ checksum في الـ job نفسه.
