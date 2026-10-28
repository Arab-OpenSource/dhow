# ADR-0056: تقوية الـ sandbox بعد المراجعة الأمنية المستقلة لـ M3.6

- **الحالة:** مقبول (قرار Sabry: «صلحهم كلهم»)
- **التاريخ:** 2026-10-28
- **بيكمّل:** [ADR-0043](0043-job-sandbox-rootless-podman.md) و[ADR-0055](0055-podman-driver-and-agent-implementation.md).

## السياق
M3.6 اندمج قبل المراجعة الأمنية المستقلة، وده كان مكتوب كبند مفتوح في `Plan/Status.md`. المراجعة ماطلّعتش أي حاجة حرجة أو عالية، لكنها طلّعت مشكلتين متوسطتين، وكمان مشاكل أقل وملاحظات. أنا بنفسي اتأكدت من المشكلتين المتوسطتين في الكود:
1. **process في الخلفية بيفضل عايش بعد خطوته.** كل الخطوات بتشتغل بنفس الـ uid، والـ agent ماكانش بيقتل حاجة بعد ما الـ shell يخلص. فخطوة 0 تقدر تسيب loop بيقرا `/run/dhow/secrets` و`/proc/*/environ`، فيوصل لأسرار خطوة 2 حتى لو السر ده متحدد لخطوة 2 بس. وكمان الـ agent كان بيكتب السر بـ `O_TRUNC` من غير `O_EXCL`، يعني ممكن السر يتكتب في FIFO أو في ملف الـ job سايبه مفتوح.
2. **`DHOW_RUNNER_DISK_MODE=directory` مالوش أي حد للمساحة.** ADR-0043 بيقول إن المجلد بيتراقب كل ثانيتين، بس المراقبة دي عمرها ما اتبنت. فـ job واحد يقدر يملا قرص الـ runner ويوقع الـ jobs التانية.

## القرار

### الخطوة بتخلص بكل الـ processes بتاعتها
- الـ agent هو PID 1 جوه الـ PID namespace بتاع الـ job. أول ما الـ shell بتاع الخطوة يخلص، الـ agent بيعمل `kill(-1, SIGKILL)` وبيعمل reap لحد `ECHILD`، **وبعدين** بيبلّغ إن الخطوة خلصت. يعني مفيش process بيعيش أكتر من خطوته، والـ zombies مابتتراكمش على `pids.max`.
- لو الـ processes ماماتتش في 10 ثواني (مثلًا process عالق في D state)، الـ agent بيقفل القناة من غير ما يبلّغ بالخطوة، والـ job بيطلع `SANDBOX_FAILED` (fail closed).
- الـ agent بيرفض يشتغل لو مش PID 1، عشان `kill(-1)` برّه الـ namespace كان هيقتل كل processes المستخدم.
- **نضحّي** بإن خدمة تبدأ في خطوة وتكمّل في اللي بعدها (زي قاعدة بيانات للاختبارات)، لصالح إن الأسرار المتحددة لخطوة تفضل فعلًا للخطوة دي بس. الخدمات اللي بتعيش طول الـ job هتيجي كميزة لوحدها (`services`) بحاويات منفصلة.
- **حدود القرار:** `/workspace` مشترك بين الخطوات، فخطوة تقدر تعدّل ملفات أو سكريبتات هتشغّلها خطوة بعدها. تحديد السر لخطوة بيمنع إن process يقراه وهو شغّال، لكنه مش بيمنع خطوة سابقة تزرع كود يشتغل جوه الخطوة اللي معاها السر. ده مكتوب في الوثائق.

### ملفات الأسرار
- قبل ما الأسرار تتكتب، الـ agent بيفضّي المجلد (ومفيش أي process من الـ job عايش في اللحظة دي). وبعدين بيكتب كل ملف بـ `openat(O_EXCL|O_NOFOLLOW)` بالنسبة لمجلد اتفتح بـ `O_DIRECTORY|O_NOFOLLOW`، والمجلد لازم يكون ملك الـ agent، وصلاحياته بترجع `0700`.
- الـ agent بيحتفظ بالـ fd من وقت الكتابة، وبيستخدمه بعد الخطوة عشان يصفّر نفس الـ inode، وبعدين بيفضّي المجلد. التصفير كده مش محتاج يفتح الملف تاني بـ `O_WRONLY`: الفتح ده كان بيفشل على ملف `0400` من غير `CAP_DAC_OVERRIDE`، وكان ممكن يعلّق على FIFO.
- الأسرار بتتمسح **قبل** ما الخطوة تتبلّغ إنها خلصت، والـ runner بيرفض أي `StepExit` رقمه مش رقم الخطوة اللي شغّالة.

### القرص: وضع `directory` مقفول
- الـ runner بيرفض `DHOW_RUNNER_DISK_MODE=directory` وقت الإقلاع (`ErrDiskUnenforced`)، لحد ما يتبني ليه حد حقيقي: quota بـ XFS project أو filesystem بـ loop لكل attempt، مش مراقبة كل ثانيتين. ده بيلغي الجزء ده من ADR-0043. الأجهزة اللي ذاكرتها قليلة تقلّل `MAX_MEMORY` و`MAX_DISK`.

### الـ image
- الـ driver بيرفض أي image مش بادية باسم الـ registry (short name). وبيسحب بـ `docker://<ref>`، وبينشئ الحاوية من الـ image ID اللي الـ pull رجّعه. كده Podman مايقدرش يفهم الـ ref على إنه transport تاني، زي `containers-storage:` اللي بيعدّي الـ registry والتوقيع، أو `docker-daemon:` اللي بيوصل لـ socket بتاع Docker على الـ host.
- **السيرفر بيكمّل الاسم القصير** في `jobspec.Normalize`، قبل الـ validation والـ digest والتوقيع (قرار Sabry): `node:22@sha256:…` بيبقى `docker.io/library/node:22@sha256:…`، وده نفس قاعدة Docker بالظبط. فالـ runner دايمًا بيستلم اسم كامل في grant موقّع، ومفيش أي بحث في registries تانية. وده مايخالفش قفل الـ short names في ADR-0043: القفل كان عشان مايحصلش بحث ممكن يوصّل لـ registry غير متوقّعة، والتكميل هنا قاعدة ثابتة، والـ digest بيثبّت المحتوى. الـ ref اللي فيه اسم transport (زي `containers-storage:alpine`) بيتكمّل لاسم على Docker Hub، يعني مابيوصلش لأي transport.
- `--unsetenv-all`: الـ environment بتاع الـ image مابيوصلش للـ agent، يعني `GODEBUG` و`GOMAXPROCS` و`GOMEMLIMIT` و`LD_*`. الخطوات أصلًا مابتورثوش. وبعد الإنشاء `verify` بيرفض أي متغير بيبدأ بـ `GO` أو `LD_`.
- `verify` بيرفض أي annotation بيقراها crun: `run.oci.*` و`module.wasm.image/*` و`io.containers.trace-*` و`org.criu.*`، سواء جت في `.Config` أو في `.HostConfig`.
- الـ driver بيرفض binary للـ agent فيه `PT_INTERP`، لأن الـ loader في الحالة دي كان هييجي من الـ image.

### الـ mounts والـ namespaces
- `default_mounts_file` بيشاور على ملف فاضي في مجلد إعدادات الـ driver. كده `/usr/share/containers/mounts.conf` على RHEL وFedora مايركّبش `/run/secrets` جوه الـ job.
- `verify` بقى أشد:
  - أي namespace لازم يكون `private` بالظبط، والقيمة الفاضية مرفوضة.
  - `no-new-privileges` لازم يكون مفعّل فعلًا، مش أي قيمة.
  - مصدر كل bind mount لازم يكون اللي الـ driver طلبه.
  - الـ tmpfs لازم تكون `nosuid,nodev`، وتبع الأسرار كمان `noexec`.
- بعد الـ start، الـ driver بيقرا `/proc/<pid>/uid_map` و`gid_map` بتوع الحاوية. وبيرفض الـ map لو فيه uid الـ runner أو root بتاع الـ host، أو لو أكبر من 65536، أو لو بيتقاطع مع حاوية تانية شغّالة.

### الـ seccomp
- `socket()` بقى مسموح بس لـ `AF_UNIX` و`AF_INET` و`AF_INET6` و`AF_NETLINK` (من غير `NETLINK_AUDIT`)، وأي family تانية بترجع `ENOSYS`. كده `AF_VSOCK` مقفول. السبب: الـ vsock مالوش namespace، فلو الـ runner VM كان الـ job هيقدر يوصل للـ hypervisor، أو لـ sshd اللي متعرّف على vsock في الـ host. الـ rules مكتوبة allow-list من غير ما تتقاطع، عشان مانعتمدش على ترتيب أولوية libseccomp.
- **حدود القرار:** `socketcall` على 32-bit لسه مسموح زي ما هو في profile بتاع Docker، لأن قفله بيكسر binaries قديمة. ولو المشكلة دي بقت مهمة، قفل الشبكة كله في M3.7 بيغطيها.

### الـ agent كـ PID 1
- الـ agent بيمسك الإشارات اللي ممكن تتبعت من جوه الـ job وبيرميها (مش `SIG_IGN`، لأن الـ ignore بيتورث للخطوات بعد `exec`، فـ `SIGTERM` مكانش هيوقف الخطوة)، والـ context بتاعه مابيتلغيش بـ `SIGTERM`. الإشارات دي هي: `TERM` و`INT` و`HUP` و`QUIT` و`USR1/2` وأخواتهم. فـ `kill -TERM 1` مثلًا مابيقفلش الـ agent، والـ kernel أصلًا مابيوصّلش `SIGKILL` لـ init من جوه الـ namespace. الإشارات المتزامنة (`SEGV` و`BUS`) لسه ممكن تقتله، وده بيأذي الـ job نفسه بس، وبيطلع `SANDBOX_FAILED` بعدد retries محدود (`MaxInfraRetries`).
- الـ runner بيستنى الـ `Hello` لحد deadline محدد.

### مجلدات الـ runner
- اختبار في الـ isolation suite بيلف على مجلد الحالة (ماعدا `storage/`، لأنه `0700`) أثناء ما في job شغّال، وبيفشل لو لقى أي حاجة مقروءة أو مكتوبة لباقي مستخدمي الجهاز. والسبب إن مجلد الحالة والـ runroot بقوا `0711` في M3.6.

- أول تشغيل على الـ CI لقى مجلد `run/intermediate-mountpoint-<uid>` بصلاحيات `0755`. Podman بيعمله عشان الـ user namespace بتاع الـ job يقدر يوصل للـ rootfs، والصلاحيات دي بتخلّي أي مستخدم على الجهاز يشوف اللي جوّاه. فالـ driver بيرجّعه `0711` بعد كل start وقبل ما الـ job يشتغل: الدخول مسموح، والعرض لأ.
- **حدود القرار:** ID الحاوية بيبان في `/proc/<pid>/cmdline` بتاع conmon لأي مستخدم على الجهاز، ومنه ممكن يوصل لملفات الـ image (مش `/workspace` ولا الأسرار، لأنهم tmpfs جوه الـ mount namespace بتاع الحاوية). الحل إن الـ runner يتنصّب على جهاز مخصّص له، أو بـ `hidepid=2` على `/proc`، وده مكتوب في وثائق التنصيب.

### الـ CI
- job الـ `isolation` بقى بيشتغل كمان لما `go.mod` أو `go.sum` أو أي حاجة في `internal/` تتغيّر، لأن الـ runner بيستورد منهم.

### probes جديدة
- process من خطوة سابقة لسه عايش (`alone`)، وloop في الخلفية بيحاول يسرق سر الخطوة اللي بعدها.
- `/run/secrets`، وsocket من نوع `AF_VSOCK`، والكتابة في `/etc/hostname` و`/run/.containerenv` و`/etc/resolv.conf`.
- image فيها annotation `run.oci.*`: يا إما `verify` يرفضها، يا إما الـ annotation ماتوصلش للحاوية.

## العواقب
- خطوة مابتقدرش تسيب process يسرق أسرار الخطوة اللي بعدها، والـ zombies بتتنضّف بين الخطوات.
- مفيش وضع قرص من غير حد. والأجهزة اللي ذاكرتها قليلة بقى عندها اختيار واحد: تقلّل الحدود.
- الأسماء القصيرة اللي في `docs/pipeline-spec.md` بقت شغّالة، وكانت قبل كده بتفشل وقت الـ pull. والـ runner لسه بيرفض أي اسم مش كامل يوصله، كخط دفاع تاني.
- **مش متغطّي:**
  - **مفيش حد للـ I/O:** `io` محتاج في الـ preflight، لكن مفيش حد بيتحط عليه. ده هيتحدد في M3.9 مع الفوضى.
  - **إطار أكبر من 1 MiB:** ممكن يحصل لو الـ env والأسرار كبار جدًا، ولسه بيطلع `SANDBOX_FAILED`.
  - **socketcall على 32-bit:** زي ما هو مكتوب فوق في الـ seccomp.
