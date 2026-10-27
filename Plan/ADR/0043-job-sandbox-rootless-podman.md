# ADR-0043: الـ sandbox: Podman rootless افتراضيًا، وagent هو PID 1، وحدود cgroup v2 بتفشل مقفولة، وgVisor اختياري

- **الحالة:** مقبول (قرار مفوَّض في M3)
- **التاريخ:** 2026-10-27
- **بيعدّل:** [ADR-0007](0007-job-isolation.md) في اختيار الـ runtime: «Docker/Podman rootless» بقى Podman rootless بس في M3.

## السياق
- ADR-0007 قرّر حاوية rootless جديدة لكل job، بحدود موارد، وrootfs للقراية بس، ومن غير شبكة ولا host mounts، وgVisor متاح في كل الإصدارات، وmicroVM في Enterprise.
- لسه محتاجين نختار الـ runtime الافتراضي، والقيم الافتراضية بالظبط، وإزاي الـ runner بيشغّل الخطوات جوه الحاوية من غير ما الأسرار تتكتب على القرص، وإزاي الحاوية تموت لو الـ runner مات.
- المرشّحين: Podman rootless، وDocker rootless، وcontainerd مع nerdctl rootless، أو إن الـ runner يشغّل OCI bundles بنفسه بـ crun أو runc في user namespace.

## القرار

### الـ runtime
- **Podman rootless، 5.4 أو أحدث:**
  - الـ runner بيكلّمه بالـ CLI بـ argv (`exec.CommandContext` و`--` قبل أي قيمة جاية من الـ job) وبيقرا `--format json`.
  - مفيش `podman system service` ولا socket خالص، فمفيش API socket ممكن يتسرّب لأي حد.
- **الـ OCI runtime:** `crun` افتراضيًا، و`runsc` لمستوى `gvisor`.
- **الإعدادات بتاعتنا، مش بتاعة الجهاز:** الـ runner بيشغّل Podman بـ `CONTAINERS_CONF` و`CONTAINERS_STORAGE_CONF` و`CONTAINERS_REGISTRIES_CONF` بتوعه في `/etc/dhow-runner/containers/`. فإعدادات `containers.conf` بتاعة الجهاز أو الـ user ما تأثرش. الـ short names مقفولة، والـ storage في `/var/lib/dhow-runner/storage` بـ overlay rootless.
- **متتحققش، اتأكد:** بعد `podman create`، الـ runner بيعمل `podman inspect` وبيتأكد من الإعدادات الفعلية:
  - الـ capabilities فاضية.
  - `no_new_privs` شغّال.
  - الـ rootfs للقراية بس.
  - ملف seccomp هو بتاعنا.
  - الشبكة `none`.
  - مفيش mount غير اللي احنا طالبينه.
  - الـ cgroup فيه الحدود.

  أي اختلاف بيفشّل الـ attempt بـ `SANDBOX_POLICY_MISMATCH` قبل ما أي كود يشتغل.
- **Linux بس.** الـ runner على Windows وmacOS مش في M3 (`docs/architecture.md` اتعدّل).

### الـ agent جوه الحاوية
- **الـ binary نفسه:** `dhow-runner` static، ومتركّب للقراية بس على `/run/dhow/agent`، وهو الـ entrypoint وPID 1 (`--init=false`).
- **قناة التحكم:** unix socket في مجلد خاص بالـ attempt على الـ host (`0700`)، متركّب جوه الحاوية. الـ runner بيقبل أول اتصال بس، والـ agent بيوصل قبل أي كود للمستخدم.
- **الخطوات:**
  - الـ runner بيبعت كل خطوة: الـ script والـ env (ومعاه الأسرار).
  - الـ agent بيشغّلها بـ `/bin/sh -e -s` والـ script على stdin، مش في argv، في process group لوحدها، وبيرجّع stdout وstderr وكود الخروج.
  - الـ env ده بيتحط للخطوة بس، مش للـ agent ولا لإعدادات Podman. فالأسرار مش بتظهر في `podman inspect` ولا في الـ storage.
- **الحماية:**
  - الـ agent بيعمل `PR_SET_DUMPABLE=0`، فمحدش جوه الحاوية يعمله ptrace.
  - لو قناة التحكم اتقفلت (الـ runner مات)، الـ agent بيخرج. ولأنه PID 1، الـ kernel بيقتل كل حاجة في الـ PID namespace. يعني مفيش job بيكمّل بعد موت الـ runner.
- **الـ logs:** Podman بـ `--log-driver=none`، فمفيش ملف log بتاع conmon على القرص. الـ output بيعدّي على قناة التحكم بس.

### القيم الافتراضية (مش قابلة للتغيير من الـ job)
| الإعداد | القيمة |
|---|---|
| المستخدم | `--userns=auto:size=65536`: كل job ليه نطاق subuid لوحده، فمفيش job يقدر يلمس ملفات job تاني حتى على الـ host |
| الصلاحيات | `--cap-drop=ALL` و`--security-opt=no-new-privileges` |
| seccomp | ملف متضمّن في الـ binary: الافتراضي بتاع containers-common، ومقفول فوقه `io_uring_*` و`bpf` و`perf_event_open` و`userfaultfd` و`keyctl` و`add_key` و`request_key` و`ptrace` و`mount` و`umount2` و`pivot_root` و`unshare` و`setns`، و`clone` بأي flag بيعمل namespace. `clone3` بيرجع `ENOSYS` عشان الـ libc يرجع لـ `clone` اللي نقدر نفلتره |
| SELinux وAppArmor | الـ labels الافتراضية (`container_t`)، وممنوع `label=disable` |
| الـ rootfs | `--read-only`. المكتوب بس: `/workspace` و`/tmp` (tmpfs بـ `nosuid,nodev`) و`/dev/shm` (64 MiB) |
| الـ mounts | الـ agent (للقراية بس) ومجلد قناة التحكم بس. مفيش host mounts، ولا socket بتاع Docker أو Podman، ولا `/var/lib/dhow-runner` |
| الـ namespaces | `--ipc=private` و`--uts=private` و`--cgroupns=private` و`--pid=private` |
| الشبكة | `--network=none`. الـ egress من خلال proxy ([ADR-0044](0044-job-network-egress-proxy.md)) |
| الأجهزة | مفيش `--device`، ومفيش `--privileged` |
| الـ ulimits | `nofile=4096` و`core=0` |
| الـ image | لازم digest (`@sha256:`). `allow_unpinned_images` في إعداد الـ runner مقفول افتراضيًا، ومش متاح من الـ job |

### الحدود (cgroup v2)
| الحد | الافتراضي | السقف (إعداد الـ runner) | التنفيذ |
|---|---|---|---|
| CPU | 1 | `max_cpu` | `--cpus` |
| الذاكرة | 1 GiB | `max_memory` | `--memory`، و`--memory-swap` بنفس القيمة (من غير swap)، والـ OOM بيترجع `OOM_KILLED` |
| الـ PIDs | 512 | `max_pids` | `--pids-limit` |
| القرص | 2 GiB | `max_disk` | `/workspace` tmpfs بـ `size=`، وده حد صارم بيتحسب من الذاكرة (شوف تحت) |
| الوقت | 30 دقيقة | 6 ساعات | timer في الـ runner، والسيرفر بيتأكد بالـ deadline ([ADR-0047](0047-job-state-machine-and-reconciliation.md)) |
| سحب الـ image | 10 دقايق | `max_pull` | timeout منفصل، ومش محسوب من وقت الـ job |

- الـ job بيطلب أقل من السقف أو قده، وأي طلب أكبر بيفشل `RESOURCE_REQUEST_EXCEEDS_RUNNER` من غير ما يتقص بصمت.
- **التفويض:** الخدمة `dhow-runner.service` بـ `Delegate=yes`، وPodman بـ `--cgroup-manager=cgroupfs` بيعمل cgroup لكل حاوية تحت الشجرة بتاعة الخدمة. `doctor` والإقلاع بيتأكدوا إن `cpu` و`memory` و`pids` و`io` مفوّضين، ولو ناقصين الـ runner **مابيقومش** (fail closed).
- **القرص:**
  - الـ tmpfs صفحاته بتتحسب على الـ memory cgroup بتاع الحاوية. فالحد الفعلي للذاكرة هو الذاكرة زائد القرص، والـ runner بيحسب كده.
  - ومع `MemorySwapMax=0` على الخدمة، صفحات الـ tmpfs (وفيها ملفات الأسرار) مابتنزلش الـ swap.
  - المشغّل يقدر يختار `disk_mode: directory` للأجهزة اللي ذاكرتها قليلة: مجلد على القرص، بيتراقب كل ثانيتين، والـ job بيتقتل لو عدّى. ده حد مرن، ومكتوب في الوثائق كده.

### الهدم (teardown)
- بعد كل attempt، مهما كانت النتيجة: `podman rm --force --volumes`، ومسح مجلد الـ attempt، والتأكد إن `podman ps --all --filter label=dhow.attempt=<id>` فاضي.
- عند إقلاع الـ runner: أي حاوية عليها `label=dhow.runner=<id>` بتتشال، وأي مجلد attempt قديم بيتمسح، قبل ما يطلب أي lease.

### gVisor
- مستوى `gvisor` = نفس الـ driver بـ `--runtime=<path>/runsc`. نسخة `runsc` مثبّتة في الـ installer، ومعاها SHA-512.
- **التحفّظ:** gVisor مع Podman rootless أقل تجربة من Docker rootful. فـ M3.8 فيها spike له شروط قبول واضحة:
  - الحدود شغّالة على الـ sandbox كله من بره.
  - الـ unix socket بتاع الـ egress شغّال.
  - الـ escape suite كلها خضرا.
  
  لو الـ spike فشل، gVisor في M3 بيبقى على Kubernetes بس (`RuntimeClass`، [ADR-0048](0048-runner-packaging-and-kubernetes.md))، وgVisor على الـ host بيتأجل بـ ADR جديد.
- البيئة تقدر تفرض حد أدنى لمستوى العزل، والـ job ما بيتأجّرش لـ runner معندوش المستوى ده.

### microVM بعدين
- الـ driver ورا interface واحد: `Prepare` و`Start` و`Exec` و`Signal` و`Kill` و`Destroy` و`Inspect`.
- Firecracker (في `ee/`، M8) هيبقى driver تالت بنفس الـ agent، وقناة التحكم على vsock بدل الـ unix socket. فالبروتوكول والـ masking والـ egress مش هيتغيّروا.

## البدائل اللي اتدرست
- **Docker rootless:** محتاج daemon لكل user وrootlesskit، والـ socket بتاعه هدف قيّم لأي حد يوصله. ومعظم الأجهزة عليها Docker rootful، وده بيدّي root لأي حد في جروب `docker`، والـ runner عمره ما هيستخدمه.
- **containerd مع nerdctl rootless:** أتقل في التنصيب (containerd وbuildkit وCNI)، وأقل انتشارًا على الـ VPS.
- **OCI bundles بـ crun على طول:** أقل اعتماديات وتحكم كامل في الـ spec. لكننا كنا هنملك سحب الـ images وفك الـ layers والـ whiteouts والتخزين، ودي أكتر حتة فيها ثغرات (path traversal في فك tar مثلًا، زي ثغرة Podman نفسها CVE-2026-17106 في 2026). لو Podman بقى عبء، الـ driver interface بيسمح نبدّله.
- **Docker rootful:** مرفوض. root.

## العواقب
- **الاعتماديات على جهاز الـ runner:** Podman 5.4+ وcrun وuidmap (`newuidmap`)، وkernel فيه cgroup v2 وuser namespaces.
  - Debian 13 وFedora وRHEL 9.6+ (وRocky وAlma) فيهم النسخة دي.
  - Debian 12 وUbuntu 24.04 من مستودعاتهم الرسمية مش مدعومين للـ runner في M3. الـ `doctor` بيقول كده بوضوح.
- **الـ capabilities متشالة كلها:** يعني `apt-get install` جوه الـ job مش هيشتغل في images كتير. الطريقة الموصى بيها image فيها اللي محتاجه. وأي capabilities إضافية (زي `CHOWN` و`SETUID` جوه الـ userns) بتيجي في M4، محكومة بسياسة البيئة، وبـ ADR.
- **الـ user namespaces المتداخلة مقفولة:** فبناء images جوه الـ job مش مدعوم في M3. ده قرار M4.
- `NoNewPrivileges` مش ممكن على `dhow-runner.service` نفسها، لأن `newuidmap` محتاج setuid أو file capabilities. التقوية على الخدمة بتتحدد بالتجربة في M3.10.
- **الـ escape suite** ([الخطة](../M3-Runner.md)) check إجباري في الـ CI، وأي فشل فيها بيوقف الـ release.
